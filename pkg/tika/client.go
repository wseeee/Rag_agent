package tika

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	endpoint   string
	httpClient *http.Client
}

func NewClient(endpoint string) *Client {
	return &Client{
		endpoint: strings.TrimRight(endpoint, "/"),
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// ExtractText 将二进制文件流发送给 Tika 抽取为纯文本（原生支持 PDF, Word, Excel, PPT 等富媒体格式）
func (c *Client) ExtractText(ctx context.Context, reader io.Reader, fileName string) (string, error) {
	url := fmt.Sprintf("%s/tika", c.endpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, reader)
	if err != nil {
		return "", fmt.Errorf("create tika request failed: %w", err)
	}

	req.Header.Set("Accept", "text/plain")
	req.Header.Set("X-Tika-OCRLanguage", "chi_sim+eng")
	if fileName != "" {
		req.Header.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileName))
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("tika extract request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("tika returned status %d: %s", resp.StatusCode, string(body))
	}

	contentBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read tika response failed: %w", err)
	}

	return strings.TrimSpace(string(contentBytes)), nil
}
