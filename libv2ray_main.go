
package libv2ray

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	coreapplog "github.com/xtls/xray-core/app/log"
	corecommlog "github.com/xtls/xray-core/common/log"
	corefilesystem "github.com/xtls/xray-core/common/platform/filesystem"
	core "github.com/xtls/xray-core/core"
	corestats "github.com/xtls/xray-core/features/stats"
	coreserial "github.com/xtls/xray-core/infra/conf/serial"
	_ "github.com/xtls/xray-core/main/distro/all"
	browser_dialer "github.com/xtls/xray-core/transport/internet/browser_dialer"
	mobasset "golang.org/x/mobile/asset"
)

const (
	coreAsset            = "xray.location.asset"
	coreCert             = "xray.location.cert"
	xudpBaseKey          = "xray.xudp.basekey"
	tunFdKey             = "xray.tun.fd"
	browserDialerAddress = "xray.browser.dialer"
	libVersion           = 40
)

type CoreController struct {
	CallbackHandler CoreCallbackHandler
	statsManager    corestats.Manager
	coreMutex       sync.Mutex
	coreInstance    *core.Instance
	IsRunning       bool
}

type CoreCallbackHandler interface {
	Startup() int
	Shutdown() int
	OnEmitStatus(int, string) int
}

type consoleLogWriter struct {
	logger *log.Logger
}

func setEnvVariable(key, value string) {
	if err := os.Setenv(key, value); err != nil {
		log.Printf("PRIME NET VPN: failed to set environment variable %s: %v", key, err)
	}
}

func InitCoreEnv(envPath string, key string) {
	if envPath != "" {
		setEnvVariable(coreAsset, envPath)
		setEnvVariable(coreCert, envPath)
	}

	if key != "" {
		setEnvVariable(xudpBaseKey, key)
	}

	corefilesystem.NewFileReader = func(path string) (io.ReadCloser, error) {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			_, file := filepath.Split(path)
			return mobasset.Open(file)
		}
		return os.Open(path)
	}
}

func NewCoreController(s CoreCallbackHandler) *CoreController {
	if err := coreapplog.RegisterHandlerCreator(
		coreapplog.LogType_Console,
		func(lt coreapplog.LogType, options coreapplog.HandlerCreatorOptions) (corecommlog.Handler, error) {
			return corecommlog.NewLogger(createStdoutLogWriter()), nil
		},
	); err != nil {
		log.Printf("PRIME NET VPN: failed to register log handler: %v", err)
	}

	return &CoreController{
		CallbackHandler: s,
	}
}

func (x *CoreController) StartLoop(configContent string, tunFd int32) error {
	if x == nil {
		return fmt.Errorf("core controller is nil")
	}

	setEnvVariable(tunFdKey, strconv.Itoa(int(tunFd)))

	x.coreMutex.Lock()
	defer x.coreMutex.Unlock()

	if x.IsRunning {
		log.Println("PRIME NET VPN: core is already running")
		return nil
	}

	return x.doStartLoop(configContent)
}

func (x *CoreController) StopLoop() error {
	if x == nil {
		return fmt.Errorf("core controller is nil")
	}

	x.coreMutex.Lock()
	defer x.coreMutex.Unlock()

	if x.IsRunning {
		x.doShutdown()

		if x.CallbackHandler != nil {
			x.CallbackHandler.Shutdown()
			x.CallbackHandler.OnEmitStatus(0, "Core stopped")
		}
	}

	return nil
}

func CheckVersionX() string {
	return fmt.Sprintf("Lib v%d, Xray-core v%s", libVersion, core.Version())
}

func ReconcileBrowserDialer(dialerAddr string) {
	setEnvVariable(browserDialerAddress, dialerAddr)
	browser_dialer.Reload()
}

func (x *CoreController) doShutdown() {
	if x.coreInstance != nil {
		if err := x.coreInstance.Close(); err != nil {
			log.Printf("PRIME NET VPN: core shutdown error: %v", err)
		}
		x.coreInstance = nil
	}

	x.IsRunning = false
	x.statsManager = nil
}

func (x *CoreController) doStartLoop(configContent string) error {
	log.Println("PRIME NET VPN: initializing Xray core")

	config, err := coreserial.LoadJSONConfig(strings.NewReader(configContent))
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	instance, err := core.New(config)
	if err != nil {
		return fmt.Errorf("core initialization failed: %w", err)
	}

	x.coreInstance = instance

	manager := instance.GetFeature(corestats.ManagerType())
	if manager == nil {
		x.doShutdown()
		return fmt.Errorf("Xray statistics manager is unavailable")
	}

	stats, ok := manager.(corestats.Manager)
	if !ok {
		x.doShutdown()
		return fmt.Errorf("invalid Xray statistics manager")
	}

	x.statsManager = stats

	log.Println("PRIME NET VPN: starting Xray core")

	if err := instance.Start(); err != nil {
		x.doShutdown()
		return fmt.Errorf("startup failed: %w", err)
	}

	x.IsRunning = true

	if x.CallbackHandler != nil {
		x.CallbackHandler.Startup()
		x.CallbackHandler.OnEmitStatus(0, "Started successfully, running")
	}

	log.Println("PRIME NET VPN: Xray core started successfully")
	return nil
}

func (w *consoleLogWriter) Write(s string) error {
	w.logger.Print(s)
	return nil
}

func (w *consoleLogWriter) Close() error {
	return nil
}

func createStdoutLogWriter() corecommlog.WriterCreator {
	return func() corecommlog.Writer {
		return &consoleLogWriter{
			logger: log.New(os.Stdout, "", 0),
		}
	}
}
