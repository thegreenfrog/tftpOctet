package tftpOctet

import (
	"fmt"
	"bytes"
	"encoding/binary"
	"strings"
)

const (
	//opcodes for the packets
	OPCODE_RRQ = uint16(1) //Read Request
	OPCODE_WRQ = uint16(2) //Write Request
	OPCODE_DATA = uint16(3) //Data 
	OPCODE_ACK = uint16(4) //Acknowledgement
	OPCODE_ERROR = uint16(5) //Error

	BLOCK_SIZE = 512 //max length of datagram
	MAX_DATAGRAM_SIZE = 516 //max length of packets
)

//-------------------------------------------------------------------------------------------------------
//Packet interface generalizes Read Request, Write Request, ACK, Data, and Error types
//to make it easier to pass packets between clients and server
//-------------------------------------------------------------------------------------------------------

type Packet interface {
	CheckPacket(b []byte) error//takes byte slice and breaks it down into packet components
	Pack() []byte//takes information and builds a byte slice that ultimately will be passed as a packet
}

type RRQ struct {
	FileName 	string
	Mode 		string
}

//gets filename and mode in packet
//check that packet has filename and mode. Return error if cannot
func ReadWritePacket(b []byte) (filename string, mode string, err error) {
	withoutOP := bytes.NewBuffer(b[2:])
	filePlusZero, err := withoutOP.ReadString(0x0)
	if err != nil {
		return "", "", err
	}
	filename = strings.TrimSpace(strings.Trim(filePlusZero, "\x00"))
	modePlusExtra, err := withoutOP.ReadString(0x0)
	if err != nil {
		return "", "", err
	}
	mode = strings.TrimSpace(modePlusExtra)
	return filename, mode, nil
}

func (r *RRQ) CheckPacket(b []byte) error{
	filename, mode, err := ReadWritePacket(b)
	r.FileName, r.Mode = filename, mode
	if err != nil {
		return err
	}
	return nil
}

//helper function to build both Read and Write Requests
func packReadWriteRQ(filename string, mode string, opcode uint16) []byte {
	buffer := new(bytes.Buffer)
	binary.Write(buffer, binary.BigEndian, opcode)
	buffer.WriteString(filename)
	buffer.WriteByte(0x0)
	buffer.WriteString(mode)
	buffer.WriteByte(0x0)
	return buffer.Bytes()
}

func (r *RRQ) Pack() []byte {
	return packReadWriteRQ(r.FileName, r.Mode, OPCODE_RRQ)
}

type WRQ struct {
	FileName 	string
	Mode 		string
}

func (w *WRQ) CheckPacket(b []byte) error {
	filename, mode, err := ReadWritePacket(b)
	w.FileName, w.Mode = filename, mode
	if err != nil {
		return err
	}
	return nil
}

func (w *WRQ) Pack() []byte {
	return packReadWriteRQ(w.FileName, w.Mode, OPCODE_WRQ)
}

type DATA struct {
	BlockNum 	uint16
	Data 		[]byte
}

func (d *DATA) CheckPacket(b []byte) error {
	if len(b) < 4 {
		return fmt.Errorf("Invalid Data packet (length = %d", len(b))
	}
	d.BlockNum = binary.BigEndian.Uint16(b[2:])
	d.Data = b[4:]
	return nil
}

func (d *DATA) Pack() []byte {
	buffer := new(bytes.Buffer)
	binary.Write(buffer, binary.BigEndian, OPCODE_DATA)
	binary.Write(buffer, binary.BigEndian, d.BlockNum)
	buffer.Write(d.Data)
	return buffer.Bytes()
}


type ACK struct {
	BlockNum uint16
}

func (a *ACK) CheckPacket(b []byte) error {
	if len(b) < 4 {
		return fmt.Errorf("Invalid ACK packet (length = %d)", len(b))
	}
	a.BlockNum = binary.BigEndian.Uint16(b[2:])
	return nil
}

func (a *ACK) Pack() []byte {
	buffer := new(bytes.Buffer)
	binary.Write(buffer, binary.BigEndian, OPCODE_ACK)
	binary.Write(buffer, binary.BigEndian, a.BlockNum)
	return buffer.Bytes()
}


type ERROR struct {
	ErrCode uint16
	ErrMsg 	string
}

func (e *ERROR) CheckPacket(b []byte) error {
	if len(b) < 4 {
		return fmt.Errorf("Invalid Error packet (length = %d)", len(b))
	}
	e.ErrCode = binary.BigEndian.Uint16(b[2:])
	end := bytes.NewBuffer(b[4:])
	endString, err := end.ReadString(0x0)
	if err != nil {
		return err
	}
	e.ErrMsg = strings.TrimSpace(endString)
	return nil
}

func (e *ERROR) Pack() []byte {
	buffer := new(bytes.Buffer)
	binary.Write(buffer, binary.BigEndian, OPCODE_ERROR)
	binary.Write(buffer, binary.BigEndian, e.ErrCode)
	buffer.WriteString(e.ErrMsg)
	buffer.WriteByte(0x0)
	return buffer.Bytes()
}

//function called before CheckPacket to identify what kind of packet byte slice is
func UnPack(buffer []byte) (Packet, error) {
	var packet Packet
	opcode := binary.BigEndian.Uint16(buffer)
	switch opcode {
		case OPCODE_RRQ: 
			packet = &RRQ{}
		case OPCODE_WRQ:
			packet = &WRQ{}
		case OPCODE_ACK:
			packet = &ACK{}
		case OPCODE_DATA:
			packet = &DATA{}
		case OPCODE_ERROR:
			packet = &ERROR{}
		default:
			return nil, fmt.Errorf("not valid opcode: %d", opcode)
	}
	return packet, packet.CheckPacket(buffer)
}