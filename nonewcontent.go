package main

import (
	"net"
	"flag"
	"os"
	"fmt"
	"strings"
)

const (
	maxUDPsize   int = 65536
	chunkTCPsize = 1024
)

var (
	v4only   bool
	v6only   bool
	channels map[string]chan bool
)

func exit() {
	status := recover()
	if status != nil {
		fmt.Fprintf(os.Stderr, "Abnormal exit: %s\n", status)
	}
}

func checkError(msg string, error os.Error) {
	if error != nil {
		panic(fmt.Sprintf("%s: %s", msg, error))
	}
}

func handleudp(conn *net.UDPConn, remaddr net.Addr, message []byte) {
	fmt.Printf("UDP packet from %s (to %s)... ", remaddr, conn.LocalAddr())
	n, error := conn.WriteTo(message, remaddr)
	checkError("Cannot write", error)
	if n != len(message) {
		panic("Cannot write")
	}
	fmt.Printf("Echoed %d bytes\n", n)
}

func handletcp(conn *net.TCPConn) {
	fmt.Printf("TCP connection from %s (to %s)... ",
		conn.RemoteAddr(), conn.LocalAddr())
	total := 0
	for {
		message := make([]byte, chunkTCPsize)
		n1, error := conn.Read(message)
		if error != nil {
			if error == os.EOF {
				break
			}
			panic("Cannot read")
		}
		n2, error := conn.Write(message[0:n1])
		checkError("Cannot write", error)
		if n2 != n1 {
			panic("Cannot write completely")
		}
		total += n2
	}
	fmt.Printf("Echoed %d bytes\n", total)
	conn.Close()
}

func waitfortcp(listener *net.TCPListener, channel chan bool) {
	for { // ever...
		conn, error := listener.AcceptTCP()
		checkError("Cannot accept", error)
		go handletcp(conn)
	}
	channel <- true
}

func waitforudp(conn *net.UDPConn, channel chan bool) {
	message := make([]byte, maxUDPsize)
	for { // ever...
		n, remaddr, error := conn.ReadFrom(message)
		checkError("Cannot read from", error)
		go handleudp(conn, remaddr, message[0:n])
	}
	channel <- true
}

// No routine in package net to test if an address is v4 or v6?
func isTCPIPv6(address *net.TCPAddr) bool { return (address.String()[0] == '[') }
func isTCPIPv4(address *net.TCPAddr) bool {
	return (address.String()[0] != '[') && (address.String()[0] != ':')
}
/* No way to have a generic function, working for both TCPAddr and UDPAddr? */
func isUDPIPv6(address *net.UDPAddr) bool { return (address.String()[0] == '[') }
func isUDPIPv4(address *net.UDPAddr) bool {
	return (address.String()[0] != '[') && (address.String()[0] != ':')
}

func listenTCP(tcptype string, addr *net.TCPAddr, listen string, mustsucceed bool) {
	listener_p, error := net.ListenTCP(tcptype, addr)
	if error != nil {
		if mustsucceed {
			panic(fmt.Sprintf("Cannot listen with \"%s\" on \"%s\": %s", tcptype, listen, error))
		} else {
			return
		}
	}
	channels[tcptype+":"+listen] = make(chan bool)
	go waitfortcp(listener_p, channels[tcptype+":"+listen])
}

func listenUDP(udptype string, addr *net.UDPAddr, listen string, mustsucceed bool) {
	// TODO: mustsucceed useless?
	connection_p, error := net.ListenUDP(udptype, addr)
	checkError(fmt.Sprintf("Cannot listen with \"%s\" on \"%s\"", udptype, listen),
		error)
	channels[udptype+":"+listen] = make(chan bool)
	go waitforudp(connection_p, channels[udptype+":"+listen])
}

func main() {
	defer exit()
	channels = make(map[string]chan bool)
	listen_p := flag.String("address", ":7", "Set the port (+optional address) to listen at")
	v4only_p := flag.Bool("4", false, "Listens only with IPv4")
	v6only_p := flag.Bool("6", false, "Listens only with IPv6")
	flag.Parse()
	v4only = *v4only_p
	v6only = *v6only_p
	listens := strings.Split(*listen_p, ",", 0)
	for _, listen := range listens {
		addr, error := net.ResolveTCPAddr(listen)
		checkError(fmt.Sprintf("Cannot parse \"%s\"", listen), error)
		if isTCPIPv6(addr) {
			if v4only {
				panic(fmt.Sprintf("IPv6 address \"%s\" with option v4-only", addr.String()))
			}
			listenTCP("tcp6", addr, listen, true)
		} else if isTCPIPv4(addr) {
			if v6only {
				panic(fmt.Sprintf("IPv4 address \"%s\" with option v6-only", addr.String()))
			}
			listenTCP("tcp4", addr, listen, true)
		} else { /* Address family unspecified.
			Ideally, listening on "tcp" should be
			sufficient. But on a Linux system where the
			sysctl variable net.ipv6.bindv6only is 1,
			listening on "tcp" only listens on IPv6
			addresses :-( And there is no easy way to find
			out that this happened. You can run "sudo
			sysctl -w net.ipv6.bindv6only=0" but you are
			not always root (and it may break other
			things). So, we call listen twice, one for
			IPv4 and one for v6. How awful. */
			if !v4only {
				addr, _ = net.ResolveTCPAddr("[::]" + listen)
				listenTCP("tcp6", addr, listen, true)
			}
			if !v6only {
				addr, _ = net.ResolveTCPAddr("0.0.0.0" + listen)
				if v4only {
					listenTCP("tcp4", addr, listen, true)
				} else {
					listenTCP("tcp4", addr, listen, false) // May fail on Linux
					// if bindv6only is 0 because the previous call to
					// listenTCP blocked all addresses (v4 and v6)
				}
			}
		}
		/* See the TCP code for the strange things we do to listen both on
		IPv4 and IPv6 */
		uaddr, error := net.ResolveUDPAddr(listen)
		if isUDPIPv6(uaddr) {
			listenUDP("udp6", uaddr, listen, true)
		} else if isUDPIPv4(uaddr) {
			listenUDP("udp4", uaddr, listen, true)
		} else { /* Address family unspecified. */
			if !v4only {
				uaddr, _ = net.ResolveUDPAddr("[::]" + listen)
				listenUDP("udp6", uaddr, listen, true)
			}
			if !v6only {
				uaddr, _ = net.ResolveUDPAddr("0.0.0.0" + listen)
				if v4only {
					listenUDP("udp4", uaddr, listen, false)
				} else {
					listenUDP("udp4", uaddr, listen, true)
				}
			}
		}
	}
	// Wait for completion of all listeners
	for listen := range channels {
		<-channels[listen]
	}
}
