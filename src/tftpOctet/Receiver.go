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
	ERROR_UNDEFINED = uint16(0)
	ERR_RECEIVE_TIMEOUT = errors.New("Receive Timeout")//variable to identify error type in testing
)

//-------------------------------------------------------------------------------------------------------
//Receiver type handles instances server is receiving data from a write from a client
//and client is receiving data from the server for a read request
//-------------------------------------------------------------------------------------------------------

type receiver struct {
	RemoteAddr *net.UDPAddr//UDP Addr to communicate with other side
	UDPConn    *net.UDPConn//access UDP port
	Writer     *io.PipeWriter//Pipe to give client data
	FileName   string//name of file for which receiving data 
	Mode       string//transfer type (octet)
	Log        *log.Logger//log to store important events
}

//initial function call
//makes continuous calls to receiveBlock until last block of data has been received
func (r *receiver) run(serverMode bool) error {

	var blockNum = uint16(1)
	var buffer []byte
	buffer = make([]byte, MAX_DATAGRAM_SIZE)
	firstBlock := true

	for {
		last, err := r.receiveBlock(buffer, blockNum, firstBlock && !serverMode)
		if err != nil {
			if r.Log != nil {
				r.Log.Printf("Error receiving block %d: %v", blockNum, err)
			}
			r.Writer.CloseWithError(err)
			return err
		}
		firstBlock = false
		if last {
			break
		}
		blockNum++
	}
	defer r.Writer.Close()
	//terminate receiver
	return nil
}

//helper function that is responsibly for sending request/ACK 
//and handles the block of data coming in from UDP port
func (r *receiver) receiveBlock(b []byte, blockNum uint16, firstBlockAndClient bool) (last bool, err error) {
	for i := 0; i < 3; i++ {
		if firstBlockAndClient {//client is sending a read request 
			readRequestPacket := RRQ{r.FileName, r.Mode}
			r.UDPConn.WriteToUDP(readRequestPacket.Pack(), r.RemoteAddr)
			r.Log.Printf("Read Request sent (%s, %d)", r.FileName, r.Mode)
		} else {//client or server is receiving data
			ackPacket := ACK{blockNum-1}
			r.UDPConn.WriteToUDP(ackPacket.Pack(), r.RemoteAddr)
			r.Log.Printf("ACK #%d sent", blockNum-1)
		}

		//give receiver a longer timeout because of latency
		setDeadlineErr := r.UDPConn.SetReadDeadline(time.Now().Add(4*time.Second))
		if setDeadlineErr != nil {
			return false, fmt.Errorf("Could not set up timeout: %v", setDeadlineErr)
		}
		for {
			dataLength, remoteAddr, readErr := r.UDPConn.ReadFromUDP(b)
			if netErr, clear := readErr.(net.Error); clear && netErr.Timeout() {
				//timeout occurred
				//package might have been lost. resend
				break
			} else if readErr != nil {
				return false, fmt.Errorf("Error reading UDP packet: %v", err)
			}
			packet, err := UnPack(b[:dataLength])
			if err != nil {//bad package. listen for another one
				continue
			}
			switch p := packet.(type) {
				case *DATA:
					r.Log.Printf("Receiver received Data #%d (%d bytes)", p.BlockNum, len(p.Data))
					if blockNum == p.BlockNum {
						if firstBlockAndClient {
							r.RemoteAddr = remoteAddr
						}
						_, err := r.Writer.Write(p.Data)
						if err == nil {
							ackPacket := ACK{blockNum}
							r.UDPConn.WriteToUDP(ackPacket.Pack(), r.RemoteAddr)
							r.Log.Printf("ACK #%d sent", blockNum)
							return len(p.Data) < BLOCK_SIZE, nil
						} else {
							r.Log.Printf("Error unpacking packet #%d", p.BlockNum)
							errPacket := ERROR{ERROR_UNDEFINED, err.Error()}
							r.UDPConn.WriteToUDP(errPacket.Pack(), r.RemoteAddr)
							return false, fmt.Errorf("Failed to Save into Memory: %v", err)
						}
					}
				case *ERROR:
					return false, fmt.Errorf("Transmission error %d: %s", p.ErrCode, p.ErrMsg)
			}
		}
	}
	return false, ERR_RECEIVE_TIMEOUT
} 

