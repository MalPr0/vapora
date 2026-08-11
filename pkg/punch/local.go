package punch

import "net"

// LocalAddr is where this side can be reached from its own network.
//
// It exists because two people behind the same router cannot reach each other
// through their public address: that would mean leaving the router and coming
// back in, which many home routers refuse to do for UDP. They see each other on
// the roster, punch at an address that goes nowhere, and stay silent while both
// talk happily to everyone outside.
//
// Nothing here is a substitute for the public address. It is the second thing
// to try, and it is the only one that works for the case the public one cannot.
func LocalAddr(port int) *net.UDPAddr {
	ip := outboundIP()
	if ip == nil {
		return nil
	}
	return &net.UDPAddr{IP: ip, Port: port}
}

// outboundIP finds the address of the interface that carries traffic out.
//
// Dialling a UDP address sends nothing — it only makes the kernel pick a route
// — so this asks the routing table the same question a real packet would,
// rather than guessing from the list of interfaces. A machine with a VPN, a
// container bridge and a wifi card has several private addresses, and only one
// of them is where a neighbour would find it.
func outboundIP() net.IP {
	conn, err := net.Dial("udp4", "192.0.2.1:9")
	if err != nil {
		return nil
	}
	defer conn.Close()

	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || addr.IP == nil {
		return nil
	}

	ip := addr.IP.To4()
	if ip == nil || ip.IsLoopback() || ip.IsUnspecified() {
		return nil
	}
	return ip
}
