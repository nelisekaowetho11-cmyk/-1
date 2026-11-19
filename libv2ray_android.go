
package libv2ray

import (
	"fmt"

	corenet "github.com/xtls/xray-core/common/net"
)

// PRIME NET VPN - Android Process Finder
//
// ProcessFinder provides Android process-finding functionality.
// Applications using AndroidLibXrayLite should implement
// FindProcessByConnection() and register it before starting Xray.

type ProcessFinder interface {
	// FindProcessByConnection finds the UID of the process
	// that owns the given connection.
	//
	// network: "tcp" or "udp"
	// srcIP: Source IP address
	// srcPort: Source port
	// destIP: Destination IP address
	// destPort: Destination port
	//
	// Returns the owning process UID, or -1 if not found.
	FindProcessByConnection(
		network, srcIP string,
		srcPort int,
		destIP string,
		destPort int,
	) int
}

// RegisterProcessFinder registers an Android process finder
// with Xray-core for per-app routing based on UID.
//
// Call before starting the Xray core.
// Pass nil to unregister the finder.
func (x *CoreController) RegisterProcessFinder(finder ProcessFinder) {
	if finder == nil {
		corenet.RegisterAndroidProcessFinder(nil)
		return
	}

	corenet.RegisterAndroidProcessFinder(
		func(
			network, srcIP string,
			srcPort uint16,
			destIP string,
			destPort uint16,
		) (uid int, name string, path string, err error) {

			// Destination information may be unavailable
			// before a connection is fully established.
			if destPort == 0 || destIP == "" {
				return 0, "", "", fmt.Errorf(
					"PRIME NET VPN processFinder: no destination for %s %s:%d",
					network, srcIP, srcPort,
				)
			}

			// Recover from panics in the Android implementation.
			defer func() {
				if r := recover(); r != nil {
					uid, name, path, err = 0, "", "",
						fmt.Errorf("processFinder panic: %v", r)
				}
			}()

			uid = finder.FindProcessByConnection(
				network,
				srcIP,
				int(srcPort),
				destIP,
				int(destPort),
			)

			return uid, fmt.Sprintf("%d", uid), "", nil
		},
	)
}
