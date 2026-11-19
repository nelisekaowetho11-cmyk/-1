
package libv2ray

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	corenet "github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/common/serial"
	core "github.com/xtls/xray-core/core"
	corestats "github.com/xtls/xray-core/features/stats"
	coreserial "github.com/xtls/xray-core/infra/conf/serial"
)

func (x *CoreController) QueryAllOutboundTrafficStats() string {
	if x == nil || x.statsManager == nil {
		return ""
	}

	var b strings.Builder

	x.statsManager.VisitCounters(func(name string, counter corestats.Counter) bool {
		parts := strings.Split(name, ">>>")
		if len(parts) != 4 || parts[0] != "outbound" || parts[2] != "traffic" {
			return true
		}

		tag := parts[1]
		direct := parts[3]
		value := counter.Set(0)

		if value <= 0 {
			return true
		}

		b.WriteString(tag)
		b.WriteByte(',')
		b.WriteString(direct)
		b.WriteByte(',')
		b.WriteString(strconv.FormatInt(value, 10))
		b.WriteByte(';')

		return true
	})

	return b.String()
}

func (x *CoreController) MeasureDelay(targetURL string) (int64, error) {
	if x == nil || !x.IsRunning || x.coreInstance == nil {
		return -1, errors.New("core is not running")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	return measureInstDelay(ctx, x.coreInstance, targetURL)
}

func MeasureOutboundDelay(configureFileContent string, targetURL string) (int64, error) {
	config, err := coreserial.LoadJSONConfig(strings.NewReader(configureFileContent))
	if err != nil {
		return -1, fmt.Errorf("config load error: %w", err)
	}

	config.Inbound = nil

	var essentialApp []*serial.TypedMessage
	for _, app := range config.App {
		if app.Type == "xray.app.proxyman.OutboundConfig" ||
			app.Type == "xray.app.dispatcher.Config" ||
			app.Type == "xray.app.log.Config" {
			essentialApp = append(essentialApp, app)
		}
	}
	config.App = essentialApp

	inst, err := core.New(config)
	if err != nil {
		return -1, fmt.Errorf("instance creation failed: %w", err)
	}

	if err := inst.Start(); err != nil {
		inst.Close()
		return -1, fmt.Errorf("startup failed: %w", err)
	}

	defer inst.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()

	return measureInstDelay(ctx, inst, targetURL)
}

func measureInstDelay(ctx context.Context, inst *core.Instance, targetURL string) (int64, error) {
	if inst == nil {
		return -1, errors.New("core instance is nil")
	}

	if targetURL == "" {
		targetURL = "https://www.google.com/generate_204"
	}

	parsedURL, err := url.ParseRequestURI(targetURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
		return -1, errors.New("invalid ping URL")
	}

	tr := &http.Transport{
		TLSHandshakeTimeout: 6 * time.Second,
		DisableKeepAlives:   true,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			dest, err := corenet.ParseDestination(fmt.Sprintf("%s:%s", network, addr))
			if err != nil {
				return nil, err
			}
			return core.Dial(ctx, inst, dest)
		},
	}

	client := &http.Client{
		Transport: tr,
		Timeout:   12 * time.Second,
	}

	defer tr.CloseIdleConnections()

	var minDuration int64 = -1
	var lastErr error
	success := false

	const attempts = 2

	for i := 0; i < attempts; i++ {
		select {
		case <-ctx.Done():
			if success {
				return minDuration, nil
			}
			return -1, ctx.Err()
		default:
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
		if err != nil {
			lastErr = fmt.Errorf("failed to create HTTP request: %w", err)
			continue
		}

		start := time.Now()
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		_, readErr := io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		if readErr != nil {
			lastErr = fmt.Errorf("failed to read response body: %w", readErr)
			continue
		}

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
			lastErr = fmt.Errorf("invalid status: %s", resp.Status)
			continue
		}

		duration := time.Since(start).Milliseconds()

		if !success || duration < minDuration {
			minDuration = duration
		}

		success = true
	}

	if !success {
		if lastErr == nil {
			lastErr = errors.New("ping failed")
		}
		return -1, lastErr
	}

	return minDuration, nil
}
