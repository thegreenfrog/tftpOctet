package tftpOctet

import (
	"testing"
	"log"
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"math/rand"
)

const(
	TRANSFER_MODE = "octet"	
)


var (
	c *Client
	s *Server
	m = map[string][]byte{}
	mutex sync.Mutex
)

//main test function
//set up the server and client and start up server
func TestMain(m *testing.M) {
	addr, _ := net.ResolveUDPAddr(UDP_NET, "localhost:3009")
	log := log.New(os.Stdout, "", log.Ldate|log.Ltime)

	s = &Server{addr, handleRead, handleWrite, log}
	go s.Startup()

	c = &Client{addr, log}

	os.Exit(m.Run())
}

//writes file to server and reads same file from server
//checks that both files are the same
func Test_Basic_Write_And_Read(t *testing.T) {
	filename := "first-write"
	mode := TRANSFER_MODE
	buffer := []byte("I want to see that this message can be written to the server byte by byte")
	c.WriteFile(filename, mode, func(w *io.PipeWriter) {
		for i := 0; i < len(buffer); i++ {
			w.Write(buffer[i:i+1])
		}
		defer w.Close()
	})
	returnBuffer := new(bytes.Buffer)
	c.ReadFile(filename, mode, func(r *io.PipeReader) {
		returnBuffer.ReadFrom(r)
	})
	if !bytes.Equal(buffer, returnBuffer.Bytes()) {
		t.Fatalf("sent: %s, received: %s", string(buffer), returnBuffer.String())
	} else {
		t.Log("%s successfully sent to server", filename)
	}
}

func Test_Check_Double_Write(t *testing.T) {
	filename := "DuplicateWrite"
	mode := TRANSFER_MODE
	bufferOne := []byte("This is a message that should not be written twice to memory")
	c.WriteFile(filename, mode, func (w *io.PipeWriter) {
		for i := 0; i < len(bufferOne); i++ {
			w.Write(bufferOne[i:i+1])
		}
		defer w.Close()
	})
	c.WriteFile(filename, mode, func (w *io.PipeWriter) {
		for i := 0; i < len(bufferOne); i++ {
			w.Write(bufferOne[i:i+1])
		}
		defer w.Close()
	})
}

func Test_Timeout(t *testing.T) {
	addr, _ := net.ResolveUDPAddr("udp", "localhost:3009")

	log := log.New(os.Stderr, "", log.Ldate|log.Ltime)

	writeHandler := func(filename string, r *io.PipeReader) {
		buf := make([]byte, 64)
		for i := 0; i < 5; i++ {
			_, err := r.Read(buf)
			if err != nil {
				panic(err)
			}
		}
		// server "fail" during receive
	}

	readHandler := func(filename string, w *io.PipeWriter) {
		for i := 0; i < 5; i++ {
			_, err := w.Write(randomByteArray(64))
			if err != nil {
				panic(err)
			}
		}
		// server "fail" during send
	}

	s = &Server{addr, readHandler, writeHandler, log}
	go s.Startup()

	c = &Client{addr, log}

	var err error
	c.WriteFile("test", "octet", func(writer *io.PipeWriter) {
		_, err = writer.Write(randomByteArray(5000))
		writer.Close()
	})
	if err != ERR_SEND_TIMEOUT {
		t.Fatalf("Send timeout expected, got %v", err)
	}

	buf := new(bytes.Buffer)
	c.ReadFile("test", "octet", func(reader *io.PipeReader) {
		_, err = buf.ReadFrom(reader)
	})
	if err != ERR_RECEIVE_TIMEOUT {
		t.Fatalf("Receive timeout expected, got %v", err)
	}
	t.Log("Success: both read and write timed out")
}

func randomByteArray(n int) []byte {
	bs := make([]byte, n)
	for i := 0; i < n; i++ {
		bs[i] = byte(rand.Int63() & 0xff)
	}
	return bs
}

//function receiver uses to handle writes to it
func handleWrite(filename string, r *io.PipeReader) {
	mutex.Lock()
	_, exists := m[filename]
	if exists {
		r.CloseWithError(fmt.Errorf("file already exists in memory: %s", filename))
		return
	}
	buffer := new(bytes.Buffer)
	datalength, err := buffer.ReadFrom(r)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read %s: %v", filename, err)
	} else {
		fmt.Fprintf(os.Stdout, "Received %s (%d bytes)", filename, datalength)
		m[filename] = append(m[filename], buffer.Bytes()...)
	}
	defer mutex.Unlock()
}

//function sender uses to send data to receiver
func handleRead(filename string, w *io.PipeWriter) {
	mutex.Lock()
	data, exists := m[filename]
	if exists {
		buffer := bytes.NewBuffer(data)
		datalength, err := buffer.WriteTo(w)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to send %s: %v", filename, err)
		} else {
			fmt.Fprintf(os.Stdout, "Sent %s (%d bytes)", filename, datalength)
		}
		w.Close()
	} else {
		w.CloseWithError(fmt.Errorf("File not found: %s", filename))
	}

	defer mutex.Unlock()
}