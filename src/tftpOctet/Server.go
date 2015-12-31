package tftpOctet

import (
	"fmt"
	"io"
	"log"
	"net"
)

//-------------------------------------------------------------------------------------------------------
//Server Type provides TFTP functionality in Octet mode. Functions tied to this type 
//are meant to be used by the server side to infinitely receive requests by clients and 
//process them appropriately
//-------------------------------------------------------------------------------------------------------


type Server struct {
	BindAddr 		*net.UDPAddr//UDP address to listen for requests form clients
	ReadHandler  	func(filename string, r *io.PipeWriter)//function provided by client that allows client to handle the file received
	WriteHandler 	func(filename string, w *io.PipeReader)//function provided by client that dicates how client is going to load file to the Pipe
	Log 			*log.Logger//Log that prints out important events of the server
}

//Only function that is can be called externally
//runs infinitely, listening for requests through the port dictated by BindAddr
func (s Server) Startup() error {
	conn, err := net.ListenUDP(UDP_NET, s.BindAddr)
	if err != nil {
		return err
	}
	for {
		err = s.handleRequest(conn)
		if err != nil {
			if s.Log != nil {
				s.Log.Printf("%v\n", err)
			}
		}
	}
}

//establish the UDP "connection" 
func (s Server) transmissionConn() (*net.UDPConn, error) {
	addr, err := net.ResolveUDPAddr(UDP_NET, ":0")
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP(UDP_NET, addr)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

//helper function that is called to handle potential requests by client 
func (s Server) handleRequest(conn *net.UDPConn) error {
	var buffer []byte
	buffer = make([]byte, MAX_DATAGRAM_SIZE)
	num, returnAddr, err := conn.ReadFromUDP(buffer)
	if err != nil {
		return fmt.Errorf("Attempt to read data from client failed: %v", err)
	}
	//create packet from data received in buffer
	packet, err := UnPack(buffer[:num])
	if err != nil {
		return err
	}
	switch p := packet.(type) {
		case *RRQ://Read Request
			s.Log.Printf("Server received read request (%s, %s)", p.FileName, p.Mode)
			transConn, err := s.transmissionConn()
			if err != nil {
				return fmt.Errorf("Attempt at transmission setup failed: %v", err)
			}
			read, write := io.Pipe()
			//set up sender type to handle sending of file to client
			send := &sender{returnAddr, transConn, read, p.FileName, p.Mode, s.Log}
			go s.ReadHandler(p.FileName, write)
			go send.run(true)
		case *WRQ://Write Request
			s.Log.Printf("Server received write request (%s, %s)", p.FileName, p.Mode)
			transConn, err := s.transmissionConn()
			if err != nil {
				return fmt.Errorf("Attempt at transmission setup failed: %v", err)
			}
			read, write := io.Pipe()
			//set up receiver type to handle receiving of file from client
			receive := &receiver{returnAddr, transConn, write, p.FileName, p.Mode, s.Log}
			go s.WriteHandler(p.FileName, read)
			go receive.run(true)
	}
	return nil
}