package app

import (
	"fmt"
	"io"
	"net"
	"strconv"
)

const PROTOCOL_VERSION = "1"
const ATTACH_ERROR_BADVERSION = 101

type Socket struct {
	sock *net.UnixConn
}

func (this *Socket) Close() error {
	return this.sock.Close()
}

func (this *Socket) Read(b []byte) (int, error) {
	return this.sock.Read(b)
}

func (this *Socket) ReadString() (string, error) {
	retval := ""
	for {
		buf := make([]byte, 1024)
		read, err := this.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			return retval, err
		}
		retval += string(buf[0 : read-1])
	}
	return retval, nil
}

func (this *Socket) Execute(cmd string, args ...string) error {
	err := this.writeString(PROTOCOL_VERSION)
	if err != nil {
		return err
	}
	err = this.writeString(cmd)
	if err != nil {
		return err
	}
	for i := 0; i < 3; i++ {
		if len(args) > i {
			err = this.writeString(args[i])
			if err != nil {
				return err
			}
		} else {
			err = this.writeString("")
			if err != nil {
				return err
			}
		}
	}

	i, err := this.readInt()
	if i != 0 {
		if i == ATTACH_ERROR_BADVERSION {
			return fmt.Errorf("Protocol mismatch with target VM")
		} else {
			return fmt.Errorf("Command failed in target VM")
		}
	}
	return err
}

func (this *Socket) readInt() (int, error) {
	b := make([]byte, 1)
	buf := make([]byte, 0)
	for {
		_, err := this.Read(b)
		if err != nil {
			return 0, err
		}
		if '0' <= b[0] && b[0] <= '9' {
			buf = append(buf, b[0])
			continue
		}

		if len(buf) == 0 {
			return 0, fmt.Errorf("cannot read int")
		} else {
			return strconv.Atoi(string(buf))
		}
	}
}

func (this *Socket) writeString(s string) error {
	return this.write([]byte(s))
}

func (this *Socket) write(bytes []byte) error {
	{
		wrote, err := this.sock.Write(bytes)
		if err != nil {
			return err
		}
		if wrote != len(bytes) {
			return fmt.Errorf("cannot write")
		}
	}
	{
		wrote, err := this.sock.Write([]byte("\x00"))
		if err != nil {
			return err
		}
		if wrote != 1 {
			return fmt.Errorf("cannot write null byte")
		}
	}
	return nil
}