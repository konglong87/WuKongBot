package enhanced

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/log"
)

// ChunkStrategy represents the strategy for chunking content
type ChunkStrategy int

const (
	ChunkByLines ChunkStrategy = iota // 按行分块
	ChunkByCharacters               // 按字符数分块
	ChunkByParagraphs               // 按段落分块
)

// ChunkConfig represents the configuration for content chunking
type ChunkConfig struct {
	MaxChunkSize      int           // 最大块大小（字符）
	Strategy          ChunkStrategy // 分块策略
	DelayBetweenChunks time.Duration // 块之间的延迟
	PreserveContext   bool          // 是否保留上下文
}

// MessageSender interface for sending messages (avoiding circular import)
type MessageSender interface {
	SendMessage(channelID, recipientID, content string) error
	Name() string
}

// ContentChunker splits long content into chunks
type ContentChunker struct {
	logger *log.Logger
}

// NewContentChunker creates a new content chunker
func NewContentChunker() *ContentChunker {
	return &ContentChunker{
		logger: log.Default(),
	}
}

// ChunkContent splits content based on the specified strategy
func (c *ContentChunker) ChunkContent(content string, config ChunkConfig) []string {
	switch config.Strategy {
	case ChunkByLines:
		return c.chunkByLines(content, config.MaxChunkSize)
	case ChunkByCharacters:
		return c.chunkByCharacters(content, config.MaxChunkSize)
	case ChunkByParagraphs:
		return c.chunkByParagraphs(content, config.MaxChunkSize)
	default:
		return c.chunkByLines(content, config.MaxChunkSize)
	}
}

// ChunkByLines splits content by lines
func (c *ContentChunker) ChunkByLines(content string, maxSize int) []string {
	return c.chunkByLines(content, maxSize)
}

// chunkByLines splits content by lines, preserving line integrity
func (c *ContentChunker) chunkByLines(content string, maxSize int) []string {
	if len(content) <= maxSize {
		return []string{content}
	}

	lines := strings.Split(content, "\n")
	var chunks []string
	var currentChunk []string
	currentSize := 0

	for _, line := range lines {
		lineSize := len(line) + 1 // +1 for newline

		// If this single line is too long, split it
		if lineSize > maxSize {
			if len(currentChunk) > 0 {
				chunks = append(chunks, strings.Join(currentChunk, "\n"))
				currentChunk = []string{}
				currentSize = 0
			}
			// Split the long line
			lineChunks := c.splitLongLine(line, maxSize)
			for i, chunk := range lineChunks {
				if i > 0 {
					chunks = append(chunks, chunk)
				} else {
					currentChunk = append(currentChunk, chunk)
					currentSize = len(chunk) + 1
				}
			}
			continue
		}

		// If adding this line would exceed max size, start a new chunk
		if currentSize+lineSize > maxSize && len(currentChunk) > 0 {
			chunks = append(chunks, strings.Join(currentChunk, "\n"))
			currentChunk = []string{line}
			currentSize = lineSize
		} else {
			currentChunk = append(currentChunk, line)
			currentSize += lineSize
		}
	}

	// Add remaining chunk
	if len(currentChunk) > 0 {
		chunks = append(chunks, strings.Join(currentChunk, "\n"))
	}

	return chunks
}

// splitLongLine splits a line that's too long
func (c *ContentChunker) splitLongLine(line string, maxSize int) []string {
	var chunks []string
	for i := 0; i < len(line); i += maxSize {
		end := i + maxSize
		if end > len(line) {
			end = len(line)
		}
		chunks = append(chunks, line[i:end])
	}
	return chunks
}

// ChunkByCharacters splits content by character count
func (c *ContentChunker) ChunkByCharacters(content string, maxSize int) []string {
	return c.chunkByCharacters(content, maxSize)
}

// chunkByCharacters splits content by exact character count
func (c *ContentChunker) chunkByCharacters(content string, maxSize int) []string {
	if len(content) <= maxSize {
		return []string{content}
	}

	var chunks []string
	for i := 0; i < len(content); i += maxSize {
		end := i + maxSize
		if end > len(content) {
			end = len(content)
		}
		chunks = append(chunks, content[i:end])
	}

	return chunks
}

// ChunkByParagraphs splits content by paragraphs
func (c *ContentChunker) ChunkByParagraphs(content string, maxSize int) []string {
	return c.chunkByParagraphs(content, maxSize)
}

// chunkByParagraphs splits content by paragraphs (blank line separated)
func (c *ContentChunker) chunkByParagraphs(content string, maxSize int) []string {
	if len(content) <= maxSize {
		return []string{content}
	}

	paragraphs := strings.Split(content, "\n\n")
	var chunks []string
	var currentChunk []string
	currentSize := 0

	for _, para := range paragraphs {
		paraSize := len(para) + 2 // +2 for paragraph separator

		if currentSize+paraSize > maxSize && len(currentChunk) > 0 {
			chunks = append(chunks, strings.Join(currentChunk, "\n\n"))
			currentChunk = []string{para}
			currentSize = paraSize
		} else {
			currentChunk = append(currentChunk, para)
			currentSize += paraSize
		}
	}

	// Add remaining chunk
	if len(currentChunk) > 0 {
		chunks = append(chunks, strings.Join(currentChunk, "\n\n"))
	}

	return chunks
}

// SendChunkedContent sends chunked content to a channel
func (c *ContentChunker) SendChunkedContent(sessionID string, content string, config ChunkConfig, sender MessageSender) error {
	if len(content) == 0 {
		return nil
	}

	// Check if chunking is needed
	if len(content) <= config.MaxChunkSize {
		return sender.SendMessage("feishu", sessionID, content)
	}

	// Split content
	chunks := c.ChunkContent(content, config)
	totalChunks := len(chunks)

	// Send each chunk
	for i, chunk := range chunks {
		var contentWithIndex string

		if totalChunks > 1 {
			contentWithIndex = fmt.Sprintf("[%d/%d]\n\n%s", i+1, totalChunks, chunk)
		} else {
			contentWithIndex = chunk
		}

		err := sender.SendMessage("feishu", sessionID, contentWithIndex)
		if err != nil {
			c.logger.Error("chunker", "failed to send chunk", "chunk", i+1, "total", totalChunks, "error", err)
			return err
		}

		// Add delay between chunks if configured
		if config.DelayBetweenChunks > 0 && i < totalChunks-1 {
			time.Sleep(config.DelayBetweenChunks)
		}
	}

	c.logger.Debug("chunker", "sent chunked content", "total_chunks", totalChunks, "session", sessionID)

	return nil
}

// FormatCodeOutput formats code output for better readability
func (c *ContentChunker) FormatCodeOutput(output string, language string) string {
	if language == "" {
		language = "bash"
	}
	return fmt.Sprintf("```%s\n%s\n```", language, output)
}

// FormatFileContent formats file content with file header
func (c *ContentChunker) FormatFileContent(filename, content string) string {
	return fmt.Sprintf("📄 **文件**: %s\n\n```\n%s\n```", filename, strings.TrimSpace(content))
}

// FormatErrorMessage formats error messages
func (c *ContentChunker) FormatErrorMessage(error string) string {
	return fmt.Sprintf("❌ **错误**: %s", error)
}

// FormatWarningMessage formats warning messages
func (c *ContentChunker) FormatWarningMessage(warning string) string {
	return fmt.Sprintf("⚠️ **警告**: %s", warning)
}

// FormatSuccessMessage formats success messages
func (c *ContentChunker) FormatSuccessMessage(message string) string {
	return fmt.Sprintf("✅ **成功**: %s", message)
}
