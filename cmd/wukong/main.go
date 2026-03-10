package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/charmbracelet/log"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"

	"github.com/konglong87/wukongbot/internal/agent"
	"github.com/konglong87/wukongbot/internal/agentteam"
	"github.com/konglong87/wukongbot/internal/bus"
	"github.com/konglong87/wukongbot/internal/channels"
	"github.com/konglong87/wukongbot/internal/config"
	"github.com/konglong87/wukongbot/internal/cron"
	"github.com/konglong87/wukongbot/internal/feishu/enhanced"
	h "github.com/konglong87/wukongbot/internal/hooks"
	"github.com/konglong87/wukongbot/internal/hooks/handlers"
	"github.com/konglong87/wukongbot/internal/providers"
	"github.com/konglong87/wukongbot/internal/session"
	"github.com/konglong87/wukongbot/internal/swagger"
	"github.com/konglong87/wukongbot/tools"
)

var (
	configPath string
	model      string
	debug      bool
	workspace  string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "wukongbot",
		Short: "wukongbot - A lightweight AI assistant",
		Long:  `wukongbot is a lightweight AI assistant framework with multi-channel support.`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if debug {
				log.SetLevel(log.DebugLevel)
			}
		},
	}

	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Path to config file")
	rootCmd.PersistentFlags().StringVar(&model, "model", "", "Model to use")
	rootCmd.PersistentFlags().StringVar(&workspace, "workspace", "", "Path to workspace directory")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug logging")

	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(chatCmd)
	rootCmd.AddCommand(scheduleCmd)
	rootCmd.AddCommand(skillsCmd)
	rootCmd.AddCommand(versionCmd)

	skillsCmd.AddCommand(skillsListCmd)
	skillsCmd.AddCommand(skillsReloadCmd)
	skillsCmd.AddCommand(skillsInfoCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the wukongbot server",
	Long:  `Start the wukongbot server with all configured channels.`,
	RunE:  runwukongbot,
}

var chatCmd = &cobra.Command{
	Use:   "chat [message]",
	Short: "Chat with the agent directly",
	Long:  `Send a message to the agent and get a response.`,
	Args:  cobra.MinimumNArgs(1),
	RunE:  runChat,
}

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Schedule a message",
	Long:  `Schedule a message to be sent later.`,
	RunE:  runSchedule,
}

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Manage skills",
	Long:  `List, reload, or get info about skills.`,
}

var skillsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available skills",
	Long:  `List all available skills from builtin and workspace directories.`,
	RunE:  runSkillsList,
}

var skillsReloadCmd = &cobra.Command{
	Use:   "reload",
	Short: "Reload all skills",
	Long:  `Clear the skills cache and re-scan all skills directories.`,
	RunE:  runSkillsReload,
}

var skillsInfoCmd = &cobra.Command{
	Use:   "info [skill-name]",
	Short: "Show skill information",
	Long:  `Show detailed information about a specific skill.`,
	Args:  cobra.MinimumNArgs(1),
	RunE:  runSkillsInfo,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("wukongbot-go 0.1.0")
		return nil
	},
}

func runwukongbot(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Resolve image API keys (fallback to providers.qwen if not set)
	cfg.ResolveImageAPIKeys()

	// Print diagnostic info
	log.Info("==================================================")
	log.Info("wukongbot CONFIGURATION DIAGNOSTIC")
	log.Info("==================================================")
	log.Info("Config file path", "path", configPath)
	log.Info("Feishu config",
		"enabled", cfg.Channels.Feishu.Enabled,
		"app_id", cfg.Channels.Feishu.AppID,
		"app_secret", "***hidden***",
		"connection_mode", cfg.Channels.Feishu.ConnectionMode)
	log.Info("Gateway config",
		"enabled", cfg.Gateway.Enabled,
		"host", cfg.Gateway.Host,
		"port", cfg.Gateway.Port)
	log.Info("==================================================")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Create message bus
	messageBus := bus.NewChannelMessageBus(100)

	// Create LLM provider
	provider, err := createProvider(cfg, model)
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	// Resolve model name (use config default if not specified via flag)
	modelName := model
	if modelName == "" {
		modelName = cfg.Agents.Defaults.Model
	}

	// Create agent configuration
	agentCfg := agent.Config{
		Workspace:             cfg.WorkspacePath(),
		Model:                 modelName,
		MaxTokens:             cfg.Agents.Defaults.MaxTokens,
		Temperature:           cfg.Agents.Defaults.Temperature,
		MaxToolIterations:     cfg.Agents.Defaults.MaxToolIterations,
		SearchProvider:        cfg.Tools.Web.Search.Provider,
		BraveAPIKey:           cfg.Tools.Web.Search.APIKey,
		ExecTimeout:           cfg.Tools.Exec.Timeout,
		ExecRestrictToWs:      cfg.Tools.Exec.RestrictToWorkspace,
		MaxHistoryMessages:    cfg.Agents.Defaults.MaxHistoryMessages,
		UseHistoryMessages:    cfg.Agents.Defaults.UseHistoryMessages,
		HistoryTimeoutSeconds: cfg.Agents.Defaults.HistoryTimeoutSeconds,
		ErrorResponse:         cfg.Agents.Defaults.ErrorResponse,
		Identity:              &cfg.Identity,
		ImageTools:            cfg.Tools.Image,
		CodeDevEnabled:        cfg.CodeDev.Enabled,
		CodeDevTimeout:        cfg.CodeDev.Timeout,
		CodeDevExecutors:      cfg.CodeDev.Executors,
	}

	// Create agent loop
	agentLoop := agent.NewAgentLoop(messageBus, provider, agentCfg)

	// Initialize enhanced handler for Feishu interactive features
	// This enables progress notifications, interactive cards, and Claude Code PTY integration
	var enhancedHandler *agent.EnhancedHandler
	if cfg.Channels.Feishu.Enabled {
		log.Info("[ENHANCED] Initializing Feishu enhanced features")

		// Get workspace from config or use default
		workspace := agentCfg.Workspace
		if workspace == "" {
			home, _ := os.UserHomeDir()
			workspace = filepath.Join(home, "wukongbot-workspace")
		}

		// Create Claude Code config from configuration
		// Prepare command string from configuration
		// claudeCommand in Config is []string
		claudeCommandDefault := []string{"opencode"}

		claudeConfig := agent.ClaudeCodeConfig{
			Enabled:        cfg.ClaudeCode.Enabled,
			Workspace:      workspace,
			SessionPrefix:  cfg.ClaudeCode.SessionPrefix,
			SessionTimeout: time.Duration(cfg.ClaudeCode.SessionTimeout) * time.Second,
			AutoCleanup:    cfg.ClaudeCode.AutoCleanup,
			MaxSessions:    cfg.ClaudeCode.MaxSessions,
			ClaudeCommand:  claudeCommandDefault,
		}

		// Use configured command if available
		if len(cfg.ClaudeCode.ClaudeCommand) > 0 {
			claudeConfig.ClaudeCommand = cfg.ClaudeCode.ClaudeCommand
		}

		// Use defaults if config is empty
		if claudeConfig.SessionPrefix == "" {
			claudeConfig.SessionPrefix = "/claude:"
		}
		if claudeConfig.SessionTimeout == 0 {
			claudeConfig.SessionTimeout = 10 * time.Minute
		}
		if claudeConfig.MaxSessions == 0 {
			claudeConfig.MaxSessions = 5
		}
		if len(claudeConfig.ClaudeCommand) == 0 {
			claudeConfig.ClaudeCommand = []string{"opencode"}
		}

		messageSender := agent.NewAgentLoopMessageSender(messageBus)
		enhancedHandler = agent.NewEnhancedHandler(messageSender, workspace, claudeConfig)
		agentLoop.SetEnhancedHandler(enhancedHandler)
		log.Info("[ENHANCED] Feishu enhanced features initialized", "claudeConfig", claudeConfig)
	}

	// Create subagent manager
	subagentCfg := agent.SubagentConfig{
		Workspace:        agentCfg.Workspace,
		Model:            agentCfg.Model,
		MaxTokens:        agentCfg.MaxTokens,
		Temperature:      agentCfg.Temperature,
		MaxIterations:    agentCfg.MaxToolIterations,
		SearchProvider:   agentCfg.SearchProvider,
		BraveAPIKey:      agentCfg.BraveAPIKey,
		ExecTimeout:      agentCfg.ExecTimeout,
		ExecRestrictToWs: agentCfg.ExecRestrictToWs,
		Identity:         &cfg.Identity,
		ImageTools:       cfg.Tools.Image,
	}
	subagentMgr := agent.NewSubagentMgr(messageBus, provider, subagentCfg)
	agentLoop.SetSubagentManager(subagentMgr)

	// Create channel manager and start channels
	channelManager := channels.NewManager(messageBus)

	var feishuChannel *channels.FeishuChannel
	var feishuHandler http.Handler

	// Register Telegram channel if configured
	if cfg.Channels.Telegram.Enabled {
		telegram := channels.NewTelegramChannel(channels.TelegramConfig{
			Enabled:   cfg.Channels.Telegram.Enabled,
			Token:     cfg.Channels.Telegram.Token,
			AllowFrom: cfg.Channels.Telegram.AllowFrom,
			Proxy:     cfg.Channels.Telegram.Proxy,
		})
		channelManager.Register(telegram)
	}

	// Register WhatsApp channel if configured
	if cfg.Channels.WhatsApp.Enabled {
		whatsapp := channels.NewWhatsAppChannel(channels.WhatsAppConfig{
			Enabled:   cfg.Channels.WhatsApp.Enabled,
			BridgeURL: cfg.Channels.WhatsApp.BridgeURL,
			AllowFrom: cfg.Channels.WhatsApp.AllowFrom,
		})
		channelManager.Register(whatsapp)
	}

	// Register Feishu channel if configured
	log.Info("Checking channels configuration...", "feishu_enabled", cfg.Channels.Feishu.Enabled)
	if cfg.Channels.Feishu.Enabled {
		log.Info("[FEISHU] Creating Feishu channel", "config", cfg.Channels.Feishu)
		feishu := channels.NewFeishuChannel(channels.FeishuConfig{
			Enabled:           cfg.Channels.Feishu.Enabled,
			AppID:             cfg.Channels.Feishu.AppID,
			AppSecret:         cfg.Channels.Feishu.AppSecret,
			EncryptKey:        cfg.Channels.Feishu.EncryptKey,
			VerificationToken: cfg.Channels.Feishu.VerificationToken,
			ConnectionMode:    cfg.Channels.Feishu.ConnectionMode,
			AllowFrom:         cfg.Channels.Feishu.AllowFrom,
		})
		feishuChannel = feishu
		feishuHandler = feishu.Handler()
		channelManager.Register(feishu)
		log.Info("[FEISHU] Channel registered successfully", "connection_mode", cfg.Channels.Feishu.ConnectionMode)
	} else {
		log.Warn("[FEISHU] Feishu channel is DISABLED in config")
	}

	// Start all channels
	log.Info("[CHANNELS] Starting all channels...")
	if err := channelManager.StartAll(ctx); err != nil {
		log.Warn("Failed to start some channels", "error", err)
	}
	log.Info("[CHANNELS] All channels start initiated")

	// Route messages from channels to bus
	channelManager.RouteMessages(ctx)

	// Route outbound messages from bus back to channels
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case outboundMsg, ok := <-messageBus.OutboundChannel():
				if !ok {
					log.Info("Outbound channel closed")
					return
				}

				// Convert Media from bus package to channels package
				media := make([]channels.Media, len(outboundMsg.Media))
				for i, m := range outboundMsg.Media {
					media[i] = channels.Media{
						Type:     m.Type,
						URL:      m.URL,
						Data:     m.Data,
						MimeType: m.MimeType,
					}
				}

				// Find the channel by channel type and send the message
				if channel, exists := channelManager.Get(outboundMsg.ChannelID); exists {
					if err := channel.Send(channels.OutboundMessage{
						ChannelID:   outboundMsg.ChannelID,
						RecipientID: outboundMsg.RecipientID,
						Content:     outboundMsg.Content,
						Type:        outboundMsg.Type,
						Metadata:    outboundMsg.Metadata,
						Media:       media,
					}); err != nil {
						log.Error("Failed to send message to channel", "channel", outboundMsg.ChannelID, "error", err)
					} else {
						log.Info("[runwukongbot] Message sent to channel", "channel", outboundMsg.ChannelID, "recipient", outboundMsg.RecipientID, "content", outboundMsg.Content, "type", outboundMsg.Type, "metadata", outboundMsg.Metadata)
					}
				} else {
					log.Warn("Channel not found for outbound message", "channel", outboundMsg.ChannelID)
				}
			}
		}
	}()

	// Create storage for cron service and session history
	storage, err := session.NewStorage(cfg)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}
	defer storage.Close()

	// Create session manager for chat history
	sessionMgr, err := session.NewManager(&session.ManagerConfig{
		Config:     cfg,
		DefaultLLM: provider,
	})
	if err != nil {
		return fmt.Errorf("failed to create session manager: %w", err)
	}
	defer sessionMgr.Close()

	// Create cron service and load existing jobs from storage
	cronService := cron.NewService(storage, messageBus, "cli", "direct")
	if err := cronService.LoadJobs(); err != nil {
		log.Warn("Failed to load cron jobs", "error", err)
	} else {
		log.Info("Loaded cron jobs from storage")
	}
	wrappedCron := &cronWrapper{service: cronService}
	agentLoop.SetCronService(wrappedCron)

	// Set session manager to agent loop for chat history
	agentLoop.SetSessionManager(&sessionManagerAdapter{sessionMgr: sessionMgr})

	// Initialize Agent Team if enabled
	var agentTeamTool *tools.AgentTeamTool
	if cfg.AgentTeam.Enabled || cfg.CodeDev.Enabled {
		log.Info("Initializing Agent Team system")

		// Create agent registry
		agentRegistry := agentteam.NewAgentRegistry()

		// Create task queue
		taskQueue := agentteam.NewTaskQueue(100)

		// Create task coordinator with provider adapter
		providerAdapter := &agentTeamProviderAdapter{provider: provider}
		taskCoordinator := agentteam.NewTaskCoordinator(agentRegistry, taskQueue, providerAdapter)

		// Create agent team tool
		agentTeamTool = tools.NewAgentTeamTool(provider, taskCoordinator, taskQueue, 10*time.Minute)

		// Register the tool
		agentLoop.GetToolRegistry().Register(agentTeamTool)

		log.Info("Agent Team initialized and tool registered", "tool_name", agentTeamTool.Name())

		// Log registered agents
		active, available := agentRegistry.GetActiveAgentsCount()
		log.Info("Agent Registry status", "active_agents", active, "available_agents", available)
	} else {
		log.Info("Agent Team disabled - not initializing")
	}

	// Initialize Hooks Registry
	hooksRegistry := h.NewHookRegistry(&cfg.Tools.Hooks)
	if cfg.Tools.Hooks.Enabled || cfg.Tools.Hooks.CodeDevelopment.Enabled {
		log.Info("Initializing hooks system")

		// Load hooks from configuration
		loadedHooks, err := h.LoadHooksFromConfig(&cfg.Tools.Hooks, agentCfg.Workspace)
		if err != nil {
			log.Error("Failed to load hooks from config", "error", err)
		} else {
			for _, hook := range loadedHooks {
				if err := hooksRegistry.Register(hook); err != nil {
					log.Error("Failed to register hook", "hook", hook.Name(), "error", err)
				} else {
					log.Info("Hook registered from config", "hook", hook.Name())
				}
			}
		}

		// Register code development monitoring hooks
		if cfg.Tools.Hooks.Enabled {
			log.Info("Registering code development monitoring hooks")

			// PreToolUse hook - logs before code_dev execution
			preCodeDevHook := handlers.NewPreCodeDevHook()
			hooksRegistry.Register(preCodeDevHook)
			log.Info("Hook registered", "name", preCodeDevHook.Name())

			// PostToolUse hook - logs after code_dev execution
			postCodeDevHook := handlers.NewPostCodeDevHook()
			hooksRegistry.Register(postCodeDevHook)
			log.Info("Hook registered", "name", postCodeDevHook.Name())

			// CodeDevProgress hook - for progress tracking
			progressHook := handlers.NewCodeDevProgressHook()
			hooksRegistry.Register(progressHook)
			log.Info("Hook registered", "name", progressHook.Name())
		}

		// Set hooks registry to agent loop
		agentLoop.SetHooksRegistry(hooksRegistry)

		// Log hooks stats
		stats := hooksRegistry.HookStats()
		log.Info("Hooks system initialized", "stats", stats)
	}

	// Initialize Swagger Registry (internal, for API tool backend)
	var swaggerRegistry *swagger.Registry
	var swaggerParser *swagger.Parser
	if len(cfg.Swagger.Sources) > 0 {
		log.Info("Initializing Swagger/OpenAPI integration")
		swaggerParser = swagger.NewParser()

		// Create Swagger config from Swagger config
		swaggerCfg := &swagger.Config{
			Sources:       make([]swagger.Source, len(cfg.Swagger.Sources)),
			MaxEndpoints:  cfg.Swagger.MaxEndpoints,
			IncludeTags:   cfg.Swagger.IncludeTags,
			ExcludeTags:   cfg.Swagger.ExcludeTags,
			DefaultLimit:  cfg.Swagger.DefaultLimit,
			DefaultOffset: cfg.Swagger.DefaultOffset,
		}
		for i, s := range cfg.Swagger.Sources {
			log.Info("Configuring Swagger source", "id", s.ID, "auth_type", s.AuthConfig.Type, "token_length", len(s.AuthConfig.Token))
			swaggerCfg.Sources[i] = swagger.Source{
				ID:              s.ID,
				Name:            s.Name,
				URL:             s.URL,
				BaseURL:         s.BaseURL,
				Enabled:         s.Enabled,
				RefreshInterval: s.RefreshInterval,
				AuthConfig: swagger.Auth{
					Type:         s.AuthConfig.Type,
					Token:        s.AuthConfig.Token,
					Username:     s.AuthConfig.Username,
					Password:     s.AuthConfig.Password,
					Headers:      s.AuthConfig.Headers,
					ClientID:     s.AuthConfig.ClientID,
					ClientSecret: s.AuthConfig.ClientSecret,
					TokenURL:     s.AuthConfig.TokenURL,
				},
			}
		}
		// Set defaults if not configured
		if swaggerCfg.MaxEndpoints == 0 {
			swaggerCfg.MaxEndpoints = 50
		}
		if swaggerCfg.DefaultLimit == 0 {
			swaggerCfg.DefaultLimit = 100
		}

		swaggerRegistry = swagger.NewRegistry(swaggerCfg, agentLoop.GetToolRegistry())
		if err := swaggerRegistry.Initialize(); err != nil {
			log.Error("Failed to initialize Swagger registry", "error", err)
		} else {
			status := swaggerRegistry.GetSourceStatus()
			for _, s := range status {
				log.Info("Swagger source loaded", "name", s.Name, "endpoints", s.Endpoints)
			}

			// Create and register SwaggerTool
			swaggerService := swagger.NewSwaggerService(swaggerRegistry, swaggerParser)
			swaggerTool := swagger.NewSwaggerTool(swaggerService)
			agentLoop.GetToolRegistry().Register(swaggerTool)
			log.Info("Swagger tool registered successfully")
		}
	}

	// Start agent loop
	go agentLoop.Start(ctx)

	// Start HTTP gateway if Feishu is enabled or gateway is configured
	var httpServer *http.Server
	var serverWg sync.WaitGroup

	if feishuChannel != nil || cfg.Gateway.Enabled {
		// Configure server
		host := cfg.Gateway.Host
		port := cfg.Gateway.Port
		if host == "" {
			host = "0.0.0.0"
		}
		if port == 0 {
			port = 8080
		}

		addr := fmt.Sprintf("%s:%d", host, port)

		// Create gin router (set to release mode)
		gin.SetMode(gin.ReleaseMode)
		router := gin.Default()

		// Register health check endpoint
		router.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status": "ok",
				"time":   time.Now().Unix(),
			})
		})

		// Register Feishu card callback handler
		if enhancedHandler != nil {
			router.POST("/feishu/card/callback", func(c *gin.Context) {
				FeishuCardCallbackHandler(c, enhancedHandler)
			})
			log.Info("Feishu card callback handler registered", "path", "/feishu/card/callback")
		}

		// Register Feishu webhook handler
		if feishuHandler != nil {
			// Wrap the net/http.Handler into gin
			router.Any("/feishu/events", func(c *gin.Context) {
				c.Request.URL.Path = "/feishu/events"
				feishuHandler.ServeHTTP(c.Writer, c.Request)
			})
			log.Info("Feishu webhook handler registered", "path", "/feishu/events")
		}

		// Register skills API handlers
		skillsGroup := router.Group("/api/skills")
		{
			skillsGroup.GET("", func(c *gin.Context) {
				SkillsAPIHandlerGET(c, agentCfg.Workspace)
			})
			skillsGroup.POST("", func(c *gin.Context) {
				SkillsAPIHandlerPOST(c, agentCfg.Workspace)
			})
		}

		// Register Swagger API handlers
		swaggerGroup := router.Group("/api/swagger")
		{
			swaggerGroup.GET("", func(c *gin.Context) {
				SwaggerAPIHandlerGET(c, swaggerRegistry)
			})
			swaggerGroup.GET("/tools", func(c *gin.Context) {
				SwaggerToolsAPIHandlerGET(c, swaggerRegistry)
			})
			swaggerGroup.POST("/reload/:id", func(c *gin.Context) {
				SwaggerReloadAPIHandlerPOST(c, swaggerRegistry)
			})
		}

		// Create HTTP server with gin router
		httpServer = &http.Server{
			Addr:         addr,
			Handler:      router,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
		}

		// Print all registered routes
		log.Info("=======================================")
		log.Info("HTTP API Routes:")
		log.Info("=======================================")
		for _, route := range router.Routes() {
			log.Info("", "method", route.Method, "path", route.Path)
		}
		log.Info("=======================================")

		// Start server in background
		serverWg.Add(1)
		go func() {
			defer serverWg.Done()
			log.Info("HTTP server started", "address", addr)
			if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Error("HTTP server error", "error", err)
			}
		}()
	}

	log.Info("wukongbot started", "model", agentCfg.Model)

	// Wait for shutdown
	select {
	case <-sigCh:
		log.Info("Shutting down...")
	case <-ctx.Done():
	}

	cancel()
	agentLoop.Stop()

	// Shutdown HTTP server
	if httpServer != nil {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Error("HTTP server shutdown error", "error", err)
		}
		serverWg.Wait()
	}

	channelManager.StopAll(ctx)
	cronService.Close()
	messageBus.Close()

	return nil
}

func runChat(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Resolve image API keys (fallback to providers.qwen if not set)
	cfg.ResolveImageAPIKeys()

	ctx := context.Background()

	// Create message bus
	messageBus := bus.NewChannelMessageBus(10)

	// Resolve model name (use config default if not specified via flag)
	modelName := model
	if modelName == "" {
		modelName = cfg.Agents.Defaults.Model
	}

	// Create LLM provider
	provider, err := createProvider(cfg, modelName)
	if err != nil {
		return fmt.Errorf("failed to create provider: %w", err)
	}

	// Create agent configuration
	agentCfg := agent.Config{
		Workspace:             cfg.WorkspacePath(),
		Model:                 modelName,
		MaxTokens:             cfg.Agents.Defaults.MaxTokens,
		Temperature:           cfg.Agents.Defaults.Temperature,
		MaxToolIterations:     cfg.Agents.Defaults.MaxToolIterations,
		SearchProvider:        cfg.Tools.Web.Search.Provider,
		BraveAPIKey:           cfg.Tools.Web.Search.APIKey,
		ExecTimeout:           cfg.Tools.Exec.Timeout,
		ExecRestrictToWs:      cfg.Tools.Exec.RestrictToWorkspace,
		MaxHistoryMessages:    cfg.Agents.Defaults.MaxHistoryMessages,
		UseHistoryMessages:    cfg.Agents.Defaults.UseHistoryMessages,
		HistoryTimeoutSeconds: cfg.Agents.Defaults.HistoryTimeoutSeconds,
		ErrorResponse:         cfg.Agents.Defaults.ErrorResponse,
		Identity:              &cfg.Identity,
		ImageTools:            cfg.Tools.Image,
		CodeDevEnabled:        cfg.CodeDev.Enabled,
		CodeDevTimeout:        cfg.CodeDev.Timeout,
		CodeDevExecutors:      cfg.CodeDev.Executors,
	}

	agentLoop := agent.NewAgentLoop(messageBus, provider, agentCfg)

	// Create session manager
	sessionMgr, err := session.NewManager(&session.ManagerConfig{
		Config:     cfg,
		DefaultLLM: provider,
	})
	if err != nil {
		return fmt.Errorf("failed to create session manager: %w", err)
	}
	defer sessionMgr.Close()
	agentLoop.SetSessionManager(&sessionManagerAdapter{sessionMgr: sessionMgr})

	// Create subagent manager
	subagentCfg := agent.SubagentConfig{
		Workspace:        agentCfg.Workspace,
		Model:            agentCfg.Model,
		MaxTokens:        agentCfg.MaxTokens,
		Temperature:      agentCfg.Temperature,
		MaxIterations:    agentCfg.MaxToolIterations,
		SearchProvider:   agentCfg.SearchProvider,
		BraveAPIKey:      agentCfg.BraveAPIKey,
		ExecTimeout:      agentCfg.ExecTimeout,
		ExecRestrictToWs: agentCfg.ExecRestrictToWs,
		Identity:         &cfg.Identity,
	}
	subagentMgr := agent.NewSubagentMgr(messageBus, provider, subagentCfg)
	agentLoop.SetSubagentManager(subagentMgr)

	// Process message
	message := args[0]
	response, err := agentLoop.ProcessDirect(ctx, message, "cli:direct", "cli", "direct")
	if err != nil {
		return fmt.Errorf("failed to process message: %w", err)
	}

	fmt.Println(response)
	return nil
}

func runSchedule(cmd *cobra.Command, args []string) error {
	fmt.Println("Scheduling messages is done via the cron tool within the agent.")
	fmt.Println("Use the 'message' tool in chat mode to set up reminders.")
	return nil
}

func runSkillsList(cmd *cobra.Command, args []string) error {
	workspace := getWorkspace()
	loader := agent.NewSkillsLoader(workspace)
	skills := loader.ListSkills(false)

	fmt.Println("Available Skills:")
	fmt.Println("=================")

	for _, skill := range skills {
		meta := loader.GetSkillMetadata(skill.Name)
		status := "available"
		if meta != nil && meta.Always {
			status = "always-loaded"
		}
		sizeKB := loader.GetSkillSize(skill.Name) / 1024
		tokens := loader.GetSkillTokenEstimate(skill.Name)
		fmt.Printf("  %s (%s, %dKB, ~%d tokens) [%s]\n",
			skill.Name, status, sizeKB, tokens, skill.Source)
		if meta != nil && meta.Description != "" {
			desc := meta.Description
			if len(desc) > 60 {
				desc = desc[:60] + "..."
			}
			fmt.Printf("      %s\n", desc)
		}
	}

	fmt.Println()
	fmt.Printf("Total: %d skills\n", len(skills))
	return nil
}

func runSkillsReload(cmd *cobra.Command, args []string) error {
	workspace := getWorkspace()
	loader := agent.NewSkillsLoader(workspace)
	loader.Reload()

	fmt.Println("Skills cache cleared and reloaded.")

	// Also list skills to verify
	skills := loader.ListSkills(false)
	fmt.Printf("Found %d skills.\n", len(skills))
	return nil
}

func runSkillsInfo(cmd *cobra.Command, args []string) error {
	workspace := getWorkspace()
	loader := agent.NewSkillsLoader(workspace)
	skillName := args[0]

	meta := loader.GetSkillMetadata(skillName)
	if meta == nil {
		return fmt.Errorf("skill '%s' not found", skillName)
	}

	size := loader.GetSkillSize(skillName)
	tokens := loader.GetSkillTokenEstimate(skillName)

	fmt.Printf("Skill: %s\n", skillName)
	fmt.Printf("  Source: %s\n", func() string {
		for _, s := range loader.ListSkills(false) {
			if s.Name == skillName {
				return s.Source
			}
		}
		return "unknown"
	}())
	fmt.Printf("  Size: %d bytes (~%dKB)\n", size, size/1024)
	fmt.Printf("  Tokens: ~%d\n", tokens)
	fmt.Printf("  Always Load: %v\n", meta.Always)
	fmt.Printf("  Description: %s\n", meta.Description)

	return nil
}

// getWorkspace returns the workspace path from config or flag
func getWorkspace() string {
	if workspace != "" {
		return workspace
	}
	if configPath != "" {
		cfg, err := config.Load(configPath)
		if err == nil {
			return cfg.WorkspacePath()
		}
	}
	// Default to ~/wukongbot-workspace
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "wukongbot-workspace")
}

// SkillsAPIHandlerGET handles GET requests to list skills
func SkillsAPIHandlerGET(c *gin.Context, workspace string) {
	loader := agent.NewSkillsLoader(workspace)
	skills := loader.ListSkills(false)

	type skillInfo struct {
		Name        string `json:"name"`
		Source      string `json:"source"`
		Size        int64  `json:"size"`
		Tokens      int    `json:"tokens"`
		AlwaysLoad  bool   `json:"always_load"`
		Description string `json:"description"`
	}

	result := make([]skillInfo, 0, len(skills))
	for _, s := range skills {
		meta := loader.GetSkillMetadata(s.Name)
		result = append(result, skillInfo{
			Name:        s.Name,
			Source:      s.Source,
			Size:        loader.GetSkillSize(s.Name),
			Tokens:      loader.GetSkillTokenEstimate(s.Name),
			AlwaysLoad:  meta != nil && meta.Always,
			Description: meta.Description,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"skills": result,
		"count":  len(result),
	})
}

// SkillsAPIHandlerPOST handles POST requests to reload skills
func SkillsAPIHandlerPOST(c *gin.Context, workspace string) {
	loader := agent.NewSkillsLoader(workspace)
	loader.Reload()
	skills := loader.ListSkills(false)

	c.JSON(http.StatusOK, gin.H{
		"status":  "reloaded",
		"count":   len(skills),
		"message": "Skills cache cleared and reloaded",
	})
}

// SwaggerAPIHandlerGET handles GET requests to list Swagger sources
func SwaggerAPIHandlerGET(c *gin.Context, registry *swagger.Registry) {
	if registry == nil {
		c.JSON(http.StatusOK, gin.H{
			"sources": []interface{}{},
			"message": "Swagger integration not enabled",
		})
		return
	}

	status := registry.GetSourceStatus()

	c.JSON(http.StatusOK, gin.H{
		"sources": status,
		"count":   len(status),
	})
}

// SwaggerToolsAPIHandlerGET handles GET requests to list Swagger tools
func SwaggerToolsAPIHandlerGET(c *gin.Context, registry *swagger.Registry) {
	if registry == nil {
		c.JSON(http.StatusOK, gin.H{
			"tools":   []string{},
			"message": "Swagger integration not enabled",
		})
		return
	}

	tools := registry.ListTools()

	c.JSON(http.StatusOK, gin.H{
		"tools": tools,
		"count": len(tools),
	})
}

// SwaggerReloadAPIHandlerPOST handles POST requests to reload a Swagger source
func SwaggerReloadAPIHandlerPOST(c *gin.Context, registry *swagger.Registry) {
	if registry == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Swagger integration not enabled",
		})
		return
	}

	sourceID := c.Param("id")
	if sourceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Source ID is required",
		})
		return
	}

	if err := registry.ReloadSource(sourceID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":    "reloaded",
		"source_id": sourceID,
	})
}

// FeishuCardCallbackHandler handles Feishu card callback POST requests
func FeishuCardCallbackHandler(c *gin.Context, enhancedHandler *agent.EnhancedHandler) {
	log.Info("[FeishuCardCallbackHandler] Entry", "remote_addr", c.Request.RemoteAddr)
	defer log.Info("[FeishuCardCallbackHandler] Exit", "remote_addr", c.Request.RemoteAddr)

	// Parse callback request from Feishu
	var callback enhanced.FeishuCardCallback
	if err := c.ShouldBindJSON(&callback); err != nil {
		log.Error("[FeishuCardCallbackHandler] Failed to parse card callback",
			"error", err,
			"remote_addr", c.Request.RemoteAddr)
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "invalid request",
		})
		return
	}

	// Log callback details
	log.Info("[FeishuCardCallbackHandler] Received card callback",
		"type", callback.Type,
		"session_id", callback.SessionID,
		"user_id", callback.UserID,
		"timestamp", callback.Timestamp)

	// Validate callback data
	if callback.SessionID == "" {
		log.Warn("[FeishuCardCallbackHandler] Missing session_id in callback")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "session_id is required",
		})
		return
	}

	if callback.UserID == "" {
		log.Warn("[FeishuCardCallbackHandler] Missing user_id in callback")
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "user_id is required",
		})
		return
	}

	// Process callback
	response, err := enhancedHandler.HandleToolCardCallback(&callback)
	if err != nil {
		log.Error("[FeishuCardCallbackHandler] Failed to handle card callback",
			"error", err,
			"session_id", callback.SessionID,
			"user_id", callback.UserID)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// Return success response
	log.Info("[FeishuCardCallbackHandler] Card callback processed successfully",
		"session_id", callback.SessionID,
		"response", response)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": response,
	})
}

func createProvider(cfg *config.Config, defaultModel string) (providers.LLMProvider, error) {
	apiKey := cfg.GetAPIKey()
	apiBase := cfg.GetAPIBase()
	modelName := defaultModel

	if modelName == "" {
		modelName = cfg.Agents.Defaults.Model
	}

	factory := providers.NewProviderFactory()
	return factory.CreateProvider(apiKey, apiBase, modelName)
}

// cronWrapper wraps cron.Service to match tools.CronService interface
type cronWrapper struct {
	service *cron.Service
}

func (w *cronWrapper) AddJob(name, scheduleKind string, everyMs int64, cronExpr, message, channel, to string, oneTime, directSend bool) *tools.JobEntry {
	job := w.service.AddJob(name, scheduleKind, everyMs, cronExpr, message, channel, to, oneTime, directSend)
	if job == nil {
		return nil
	}
	// Convert cron.JobEntry to tools.JobEntry
	return &tools.JobEntry{
		ID:             job.ID,
		Name:           job.Name,
		Schedule:       tools.CronSchedule{Kind: job.Schedule.Kind, EveryMs: job.Schedule.EveryMs, CronExpr: job.Schedule.CronExpr},
		Message:        job.Message,
		Channel:        job.Channel,
		To:             job.To,
		DeleteAfterRun: job.DeleteAfterRun,
		CronExpr:       job.CronExpr,
		EntryID:        int(job.EntryID),
	}
}

func (w *cronWrapper) UpdateJob(id string, scheduleKind string, everyMs int64, cronExpr, message string, oneTime, directSend bool) *tools.JobEntry {
	job := w.service.UpdateJob(id, scheduleKind, everyMs, cronExpr, message, oneTime, directSend)
	if job == nil {
		return nil
	}
	return &tools.JobEntry{
		ID:             job.ID,
		Name:           job.Name,
		Schedule:       tools.CronSchedule{Kind: job.Schedule.Kind, EveryMs: job.Schedule.EveryMs, CronExpr: job.Schedule.CronExpr},
		Message:        job.Message,
		Channel:        job.Channel,
		To:             job.To,
		DeleteAfterRun: job.DeleteAfterRun,
		CronExpr:       job.CronExpr,
		EntryID:        int(job.EntryID),
	}
}

func (w *cronWrapper) ListJobs() []*tools.JobEntry {
	jobs := w.service.ListJobs()
	result := make([]*tools.JobEntry, len(jobs))
	for i, j := range jobs {
		result[i] = &tools.JobEntry{
			ID:             j.ID,
			Name:           j.Name,
			Schedule:       tools.CronSchedule{Kind: j.Schedule.Kind, EveryMs: j.Schedule.EveryMs, CronExpr: j.Schedule.CronExpr},
			Message:        j.Message,
			Channel:        j.Channel,
			To:             j.To,
			DeleteAfterRun: j.DeleteAfterRun,
			CronExpr:       j.CronExpr,
			EntryID:        int(j.EntryID),
		}
	}
	return result
}

func (w *cronWrapper) RemoveJob(id string) bool {
	return w.service.RemoveJob(id)
}

func (w *cronWrapper) EnableJob(id string) bool {
	return w.service.EnableJob(id)
}

func (w *cronWrapper) DisableJob(id string) bool {
	return w.service.DisableJob(id)
}

func (w *cronWrapper) GetExecutionLogs(jobID string, limit int) string {
	return w.service.GetExecutionLogs(jobID, limit)
}

// sessionManagerAdapter adapts session.Manager to agent.SessionManager interface
type sessionManagerAdapter struct {
	sessionMgr *session.Manager
}

func (a *sessionManagerAdapter) GetSessionMessagesAsLLMMessages(sessionKey string, limit int) ([]agent.LLMMessage, error) {
	messages, err := a.sessionMgr.GetSessionMessagesWithTimestamp(context.Background(), sessionKey, limit)
	if err != nil {
		return nil, err
	}
	result := make([]agent.LLMMessage, len(messages))
	for i, m := range messages {
		result[i] = agent.LLMMessage{Role: m.Role, Content: m.Content, Timestamp: m.Timestamp}
	}
	// Debug logging with full content and function name
	log.Info("[sessionManagerAdapter.GetSessionMessagesAsLLMMessages] Total messages loaded from database",
		"session_key", sessionKey,
		"limit", limit,
		"actual_count", len(messages))
	for i, msg := range messages {
		log.Info("[sessionManagerAdapter.GetSessionMessagesAsLLMMessages] Message",
			"session_key", sessionKey,
			"index", i,
			"role", msg.Role,
			"content", msg.Content,
			"timestamp", msg.Timestamp)
	}
	return result, nil
}

func (a *sessionManagerAdapter) AddMessage(sessionKey string, role, content string) error {
	sess, err := a.sessionMgr.GetOrCreateSession(context.Background(), sessionKey)
	if err != nil {
		return err
	}
	return a.sessionMgr.AddMessage(context.Background(), sess.ID, role, content)
}
