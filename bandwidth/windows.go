// +build windows

package bandwidth

import (
	"net"
	"sort"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	MaxStringSize         = 256
	MaxPhysicalAddrLength = 32
	pad0for64_4for32      = 0
)

var kernel32 = syscall.NewLazyDLL("kernel32.dll")
var user32 = syscall.NewLazyDLL("user32.dll")
var iphlpapi = syscall.NewLazyDLL("iphlpapi.dll")
var procGetIfEntry2 = iphlpapi.NewProc("GetIfEntry2")

var statsEnabled bool
var lastStatsTime = time.Now()

type EthrNetStat struct {
	NetDevStats []EthrNetDevStat
}

type guid struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

type mibIfRow2 struct {
	InterfaceLuid               uint64
	InterfaceIndex              uint32
	InterfaceGUID               guid
	Alias                       [MaxStringSize + 1]uint16
	Description                 [MaxStringSize + 1]uint16
	PhysicalAddressLength       uint32
	PhysicalAddress             [MaxPhysicalAddrLength]uint8
	PermanentPhysicalAddress    [MaxPhysicalAddrLength]uint8
	Mtu                         uint32
	Type                        uint32
	TunnelType                  uint32
	MediaType                   uint32
	PhysicalMediumType          uint32
	AccessType                  uint32
	DirectionType               uint32
	InterfaceAndOperStatusFlags uint32
	OperStatus                  uint32
	AdminStatus                 uint32
	MediaConnectState           uint32
	NetworkGUID                 guid
	ConnectionType              uint32
	padding1                    [pad0for64_4for32]byte
	TransmitLinkSpeed           uint64
	ReceiveLinkSpeed            uint64
	InOctets                    uint64
	InUcastPkts                 uint64
	InNUcastPkts                uint64
	InDiscards                  uint64
	InErrors                    uint64
	InUnknownProtos             uint64
	InUcastOctets               uint64
	InMulticastOctets           uint64
	InBroadcastOctets           uint64
	OutOctets                   uint64
	OutUcastPkts                uint64
	OutNUcastPkts               uint64
	OutDiscards                 uint64
	OutErrors                   uint64
	OutUcastOctets              uint64
	OutMulticastOctets          uint64
	OutBroadcastOctets          uint64
	OutQLen                     uint64
}

type EthrNetDevStat struct {
	InterfaceName string
	RxBytes       uint64
	TxBytes       uint64
	RxPkts        uint64
	TxPkts        uint64
}

type EthrNetDevInfo struct {
	Bytes   uint64
	Packets uint64
	Drop    uint64
	Errs    uint64
}

func GetNetDevStats(stats *EthrNetStat) {
	ifs, err := net.Interfaces()
	if err != nil {
		panic(err)
	}

	for _, ifi := range ifs {
		if (ifi.Flags&net.FlagUp) == 0 || strings.Contains(ifi.Name, "Pseudo") {
			continue
		}
		row := mibIfRow2{InterfaceIndex: uint32(ifi.Index)}
		e := getIfEntry2(&row)
		if e != nil {
			panic(err)
		}
		rxInfo := EthrNetDevInfo{
			Bytes:   uint64(row.InOctets),
			Packets: uint64(row.InUcastPkts),
			Drop:    uint64(row.InDiscards),
			Errs:    uint64(row.InErrors),
		}
		txInfo := EthrNetDevInfo{
			Bytes:   uint64(row.OutOctets),
			Packets: uint64(row.OutUcastPkts),
			Drop:    uint64(row.OutDiscards),
			Errs:    uint64(row.OutErrors),
		}
		netStats := EthrNetDevStat{
			InterfaceName: ifi.Name,
			RxBytes:       rxInfo.Bytes,
			TxBytes:       txInfo.Bytes,
			RxPkts:        rxInfo.Packets,
			TxPkts:        txInfo.Packets,
		}
		stats.NetDevStats = append(stats.NetDevStats, netStats)
	}
}

func getIfEntry2(row *mibIfRow2) (errcode error) {
	r0, _, _ := syscall.Syscall(procGetIfEntry2.Addr(), 1,
		uintptr(unsafe.Pointer(row)), 0, 0)
	if r0 != 0 {
		errcode = syscall.Errno(r0)
	}
	return
}

func GetNetworkStats() EthrNetStat {
	stats := &EthrNetStat{}

	GetNetDevStats(stats)
	sort.SliceStable(stats.NetDevStats, func(i, j int) bool {
		return stats.NetDevStats[i].InterfaceName < stats.NetDevStats[j].InterfaceName
	})

	return *stats
}

func getNetDevStatDiff(curStats EthrNetDevStat, prevNetStats EthrNetStat, seconds uint64) EthrNetDevStat {
	for _, prevStats := range prevNetStats.NetDevStats {
		if prevStats.InterfaceName != curStats.InterfaceName {
			continue
		}

		if curStats.RxBytes >= prevStats.RxBytes {
			curStats.RxBytes -= prevStats.RxBytes
		} else {
			curStats.RxBytes += (^uint64(0) - prevStats.RxBytes)
		}

		if curStats.TxBytes >= prevStats.TxBytes {
			curStats.TxBytes -= prevStats.TxBytes
		} else {
			curStats.TxBytes += (^uint64(0) - prevStats.TxBytes)
		}

		if curStats.RxPkts >= prevStats.RxPkts {
			curStats.RxPkts -= prevStats.RxPkts
		} else {
			curStats.RxPkts += (^uint64(0) - prevStats.RxPkts)
		}

		if curStats.TxPkts >= prevStats.TxPkts {
			curStats.TxPkts -= prevStats.TxPkts
		} else {
			curStats.TxPkts += (^uint64(0) - prevStats.TxPkts)
		}

		break
	}
	curStats.RxBytes /= seconds
	curStats.TxBytes /= seconds
	curStats.RxPkts /= seconds
	curStats.TxPkts /= seconds
	return curStats
}

func timeToNextTick() time.Duration {
	nextTick := lastStatsTime.Add(time.Second)
	return time.Until(nextTick)
}

type NetworkDeviceBandwidth struct {
	Name string `json:"name"`
	Rx   uint64 `json:"rx"`
	Tx   uint64 `json:"tx"`
}

func Check() []NetworkDeviceBandwidth {
	stats := GetNetworkStats()
	var networkDevices []NetworkDeviceBandwidth
	for _, dev := range stats.NetDevStats {
		n := NetworkDeviceBandwidth{
			Name: dev.InterfaceName,
			Rx:   dev.RxBytes,
			Tx:   dev.TxBytes,
		}
		networkDevices = append(networkDevices, n)

	}
	return networkDevices
}
