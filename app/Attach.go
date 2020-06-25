package app

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

type VirtualMachine struct {
	Pid          int32   // 目标进程 PID
	SocketPath   string  // .java_pidXXX 文件路径
	Socket       *Socket // Socket 连接
	SockFileLock sync.RWMutex // socketFile 的锁，线程安全的将sockPath置为空字符串
}

func NewVirtualMachine(pid int32) *VirtualMachine {
	socketPath := filepath.Join(os.TempDir(), fmt.Sprintf(".java_pid%d", pid))
	return &VirtualMachine{
		Pid:        pid,
		SocketPath: socketPath,
	}
}

// 与目标 JVM 建立连接
func (this *VirtualMachine) Attach() error {
	if !this.existsSocketFile() {
		err := this.createAttachFile()
		if err != nil {
			return err
		}
		err = syscall.Kill(int(this.Pid), syscall.SIGQUIT)
		if err != nil {
			return fmt.Errorf("Canot send sigquit to java[%d],%v", this.Pid, err)
		}
		// 判断文件是否创建
		count := 0
		for {
			count++
			time.Sleep(200 * time.Millisecond)
			// 文件不存在时
			if !this.existsSocketFile() && count < 25 {
				err = syscall.Kill(int(this.Pid), syscall.SIGQUIT)
				if err != nil {
					return fmt.Errorf("Canot send sigquit to java[%d],%v", this.Pid, err)
				}
				continue
			} else {
				break
			}
		}
		if !this.existsSocketFile() {
			return fmt.Errorf("attach err")
		}
	}

	// 监听 connet
	addr, err := net.ResolveUnixAddr("unix", this.SocketPath)
	if err != nil {
		return err
	}

	c, err := net.DialUnix("unix", nil, addr)
	if err != nil {
		return err
	}
	this.Socket = &Socket{c}
	return nil
}

func (this *VirtualMachine) Detach() {
	this.SockFileLock.Lock()
	defer this.SockFileLock.Unlock()
	if this.SocketPath != "" {
		this.SocketPath = ""
	}
}

func (this *VirtualMachine) existsSocketFile() bool {
	_, err := os.Stat(this.SocketPath)
	if err != nil {
		return false
	}
	return true
}

func (this *VirtualMachine) createAttachFile() error {
	attachFile := filepath.Join(os.TempDir(), fmt.Sprintf(".attach_pid%d", this.Pid))
	f, err := os.Create(attachFile)
	if err != nil {
		return fmt.Errorf("Canot create attachfile,%v", err)
	}
	fmt.Printf("attach_pid=%s", attachFile)
	defer f.Close()
	return nil
}

func (this *VirtualMachine) LoadAgent(agentJarPath string) error {
	this.SockFileLock.Lock()
	if this.SocketPath == "" {
		return fmt.Errorf("Detach has run")
	}
	this.SockFileLock.Unlock()

	err := this.Socket.Execute("load", "instrument", "false", agentJarPath)
	if err != nil {
		return fmt.Errorf("execute load %s,  %v", agentJarPath, err)
	}
	s, err := this.Socket.ReadString()
	if err != nil {
		return err
	}
	//如果发送 agent jar 包成功，
	if s != "0" {
		return fmt.Errorf("Attach Fail")
	}
	defer func() {
		_ = this.Socket.Close()
	}()
	return nil
}
