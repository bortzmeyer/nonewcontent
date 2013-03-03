package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
)

const (
	maxUDPsize   int = 65536
	chunkTCPsize     = 1024
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

func checkError(msg string, err error) {
	if err != nil {
		panic(fmt.Sprintf("%s: %s", msg, err))
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
		n1, err := conn.Read(message)
		if err != nil {
			if err == io.EOF {
				break
			}
			panic("Cannot read")
		}
		n2, err := conn.Write(message[0:n1])
		checkError("Cannot write", err)
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
func isTCPIPv6(address *net.TCPAddr) bool {
	if strings.HasPrefix(address.String(), "<nil>") {
		return false
	} else {
		return (address.String()[0] == '[')
	}
	return false // Should never arrive here
}
func isTCPIPv4(address *net.TCPAddr) bool {
	if strings.HasPrefix(address.String(), "<nil>") {
		return false
	} else {
		return (address.String()[0] != '[') && (address.String()[0] != ':')
	}
	return false // Should never arrive here
}

/* No way to have a generic function, working for both TCPAddr and UDPAddr? */
func isUDPIPv6(address *net.UDPAddr) bool {
	if strings.HasPrefix(address.String(), "<nil>") {
		return false
	} else {
		return (address.String()[0] == '[')
	}
	return false // Should never arrive here
}
func isUDPIPv4(address *net.UDPAddr) bool {
	if strings.HasPrefix(address.String(), "<nil>") {
		return false
	} else {
		return (address.String()[0] != '[') && (address.String()[0] != ':')
	}
	return false // Should never arrive here
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
	listens := strings.Split(*listen_p, ",")
	for _, listen := range listens {
		addr, err := net.ResolveTCPAddr("tcp", listen)
		checkError(fmt.Sprintf("Cannot parse \"%s\"", listen), err)
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
		} else { // Address family unspecified. 
			if !v4only && !v6only {
				listenTCP("tcp", addr, listen, true)
			} else {
				if v6only {
					addr, _ = net.ResolveTCPAddr("tcp", "[::]"+listen)
					listenTCP("tcp6", addr, listen, true)
				}
				if v4only {
					addr, _ = net.ResolveTCPAddr("tcp", "0.0.0.0"+listen)
					listenTCP("tcp4", addr, listen, true)
				}
			}
		}
		uaddr, err := net.ResolveUDPAddr("udp", listen)
		if isUDPIPv6(uaddr) {
			listenUDP("udp6", uaddr, listen, true)
		} else if isUDPIPv4(uaddr) {
			listenUDP("udp4", uaddr, listen, true)
		} else { /* Address family unspecified. */
			if !v4only && !v6only {
				listenUDP("udp", uaddr, listen, true)
			} else {
				if v4only {
					uaddr, _ = net.ResolveUDPAddr("udp", "0.0.0.0"+listen)
					listenUDP("udp4", uaddr, listen, true)
				}
				if v6only {
					uaddr, _ = net.ResolveUDPAddr("udp", "[::]"+listen)
					listenUDP("udp6", uaddr, listen, true)
				}
			}
		}
	}
	// Wait for completion of all listeners
	for listen := range channels {
		<-channels[listen]
	}
}
