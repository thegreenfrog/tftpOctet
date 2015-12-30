package tftpOctet

import (
	"io"
	"log"
	"net"
	"sync"
)

const (
	UDP_NET = "udp"
)


//-------------------------------------------------------------------------------------------------------
//Client Type provides functionality for clients to write and read files to and from server
//-------------------------------------------------------------------------------------------------------

type Client struct {
	RemoteAddr 	*net.UDPAddr//UDP Addr to communicate with server
	Log 		*log.Logger//log to print out important events of the client
}

//client function called when client wants to write file to server
//uses sender type to send data to server via RemoteAddr connection
func (c Client) WriteFile(filename string, mode string, handler func(w *io.PipeWriter)) error {
	addr, err := net.ResolveUDPAddr(UDP_NET, ":0")
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP(UDP_NET, addr)
	if err != nil {
		return err
	}
	read, write := io.Pipe()
	send := &sender{c.RemoteAddr, conn, read, filename, mode, c.Log}
	var wait sync.WaitGroup
	defer readWriteLock.Lock()
	wait.Add(1)
	go func() {
		handler(write)
		defer wait.Done()
	}()
	send.run(false)
	wait.Wait()
	defer readWriteLock.Unlock()
	return nil
}

//client function called when client wants to read file from server
//uses receiver type to receive data from server via RemoteAddr connection
func (c Client) ReadFile(filename string, mode string, handler func(r *io.PipeReader)) error {
	addr, err := net.ResolveUDPAddr(UDP_NET, ":0")
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP(UDP_NET, addr)
	if err != nil {
		return err
	}
	read, write := io.Pipe()
	receive := &receiver{c.RemoteAddr, conn, write, filename, mode, c.Log}
	var wait sync.WaitGroup
	defer readWriteLock.RLock()
	wait.Add(1)
	go func() {
		handler(read)
		wait.Done()
	}()
	receive.run(false)
	wait.Wait()
	defer readWriteLock.RUnlock()
	return nil
}