package tftpOctet

import (
	"fmt"
	"net"
	"io"
	"log"
	"time"
	"errors"
)

var (
	ERR_SEND_TIMEOUT = errors.New("Send Timeout")
)

//-------------------------------------------------------------------------------------------------------
//Sender type handles instances server is sending files to the client
//and clients are writing files to the server
//-------------------------------------------------------------------------------------------------------

type sender struct {
	RemoteAddr *net.UDPAddr//UDP Addr to communicate with other side
	UDPConn    *net.UDPConn//access UDP port
	Reader     *io.PipeReader//Pipe to get data from sender
	FileName   string//name of file for which receiving data 
	Mode       string//transfer type (octet)
	Log        *log.Logger//log to store important events
}

//initial function call
//sends initial write request if client or immediately starts sending data packets
func (s *sender) run(serverMode bool) {
	var buffer, dataGram []byte
	buffer = make([]byte, BLOCK_SIZE)
	dataGram = make([]byte, MAX_DATAGRAM_SIZE)

	//client needs to send WRQ first
	if !serverMode {
		err := s.sendWriteRequest(dataGram)
		if err != nil {
			s.Log.Printf("Error starting transmission: %v", err)
			s.Reader.CloseWithError(err)
			return
		}
	}
	//received ACK to proceed with write
	
	var blockNum = uint16(1)
	//keep sending packets until reach end of file
	for {
		dataLength, readErr := io.ReadFull(s.Reader, buffer)
		if readErr != nil && readErr != io.ErrUnexpectedEOF {
			//Error in case of EOF is actually not an error.
			//file block size was 0, send 0-sized block to signal terminate
			if readErr == io.EOF {
				sendErr := s.sendPackets(buffer, 0, blockNum, dataGram)
				if sendErr != nil && s.Log != nil {
					s.Log.Printf("Error sending last zero-sized block: %v", sendErr)
				}
			} else {//unexpected EOF
				if s.Log != nil {
					s.Log.Printf("Handler error: %v", readErr)
				}
				errPacket := ERROR{1, readErr.Error()}
				s.UDPConn.WriteToUDP(errPacket.Pack(), s.RemoteAddr)
				s.Log.Printf("sent ERROR %d: %s", 1, readErr.Error())
			}
			s.Reader.Close()
			return 
		}
		sendErr := s.sendPackets(buffer, dataLength, blockNum, dataGram)
		if sendErr != nil {
			if s.Log != nil {
				s.Log.Printf("Error sending block %d: %v", blockNum, sendErr)
			}
			s.Reader.CloseWithError(sendErr)
			return
		}
		blockNum++
		if dataLength < len(buffer) {
			//last packet was not full, so that means EOF
			defer s.Reader.Close()
			return
		}
	}
}

//send write request to server from client
func (s *sender) sendWriteRequest(dataGram []byte) error {
	//allow for three attempts at sending request
	for i:=0; i < 3; i++ {
		writePacket := WRQ{s.FileName, s.Mode}
		s.UDPConn.WriteToUDP(writePacket.Pack(), s.RemoteAddr)
		s.Log.Printf("Write Request Sent (%s, %s)", s.FileName, s.Mode)
		setDeadlineErr := s.UDPConn.SetReadDeadline(time.Now().Add(3*time.Second))
		if setDeadlineErr != nil {
			return fmt.Errorf("Failed to set up packet timeout: %v", setDeadlineErr)
		}

		for {
			dataLength, remoteAddress, readErr := s.UDPConn.ReadFromUDP(dataGram)
			if netErr, clear := readErr.(net.Error); clear && netErr.Timeout() {//timeout. Resend
				break
			} else if readErr != nil {
				return fmt.Errorf("Error reading UDP packet: %v", readErr)
			}
			packet, err := UnPack(dataGram[:dataLength])
			if err != nil {//bad packet, listen for another one
				continue
			}
			switch p := packet.(type) {
				case *ACK:
					if p.BlockNum == 0 {
						s.Log.Printf("Sender received ACK 0")
						s.RemoteAddr = remoteAddress
						return nil
					}
				case *ERROR:
					return fmt.Errorf("Transmission Error %d: %s", p.ErrCode, p.ErrMsg)
			}
		}
	}
	return ERR_SEND_TIMEOUT
}

//send block to server. 
//Return error if send fails or if received error message instead of ack from server
func (s *sender) sendPackets(b []byte, dataLength int, blockNum uint16, dataGram []byte) error {
	//allow for three attempts at sending packet
	for i := 0; i < 3; i++ {
		setDeadlineErr := s.UDPConn.SetReadDeadline(time.Now().Add(3*time.Second))
		if setDeadlineErr != nil {
			return fmt.Errorf("Failed to set up packet timeout: %v", setDeadlineErr)
		}

		dataPack := DATA{blockNum, b[:dataLength]}
		s.UDPConn.WriteToUDP(dataPack.Pack(), s.RemoteAddr)
		s.Log.Printf("Sent data packet #%d", blockNum)

		//wait for response from client
		for {
			dataLength, _, readErr := s.UDPConn.ReadFromUDP(dataGram)
			if netErr, clear := readErr.(net.Error); clear && netErr.Timeout() {//timeout. Resend
				break 
			} else if readErr != nil {
				return fmt.Errorf("Error reading UDP packet: %v", readErr)
			}
			packet, err := UnPack(dataGram[:dataLength])
			if err != nil { //bad packet, wait for another one
				continue
			}
			switch p := packet.(type) {
				case *ACK:
					s.Log.Printf("Sender received ACK %d", p.BlockNum)
					if blockNum == p.BlockNum { //successful
						return nil
					}
				case *ERROR:
					return fmt.Errorf("Transmit Error %d: %s", p.ErrCode, p.ErrMsg)
			}
		}
	}
	return ERR_SEND_TIMEOUT	
}