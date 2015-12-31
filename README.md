# TFTP server and client in Golang
To use Server/Client instances: `import "github.com/thegreenfrog/tftpOctet"`

# Server
Starting up the server:
```
addr, _ := net.ResolveUDPAddr(UDP_NET, "localhost:PORT")
log := log.New(os.Stderr, "", log.Ldate|log.Ltime)

s *Server;
s = &Server{addr, handleRead, handleWrite, log}
go s.Startup()
```
Where `handleRead` is a function used to send file data to the client and `handleWrite` is a function used to handle files being written to the server.

# Client
Starting up a client instance:
```
addr, _ := net.ResolveUDPAddr(UDP_NET, "localhost:PORT")
log := log.New(os.Stderr, "", log.Ldate|log.Ltime)
c *Client
c = &Client{addr, log}
filename := "first-write"
mode := TRANSFER_MODE
buffer := []byte("I want to see that this message can be written to the server byte by byte")
//Write File to Server
c.WriteFile(filename, mode, func(w *io.PipeWriter) {
	for i := 0; i < len(buffer); i++ {
		w.Write(buffer[i:i+1])
	}
	defer w.Close()
})
returnBuffer := new(bytes.Buffer)
//Read File from Server
c.ReadFile(filename, mode, func(r *io.PipeReader) {
	returnBuffer.ReadFrom(r)
})
```
