package netrans

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
)

// DataFrame 数据帧的结构体
type DataFrame struct {
	Length uint32 // 数据长度（前四字节）
	Obs    byte   // 混淆因子（第五字节）
	Data   []byte // 数据（其余字节）
}

// NewDataFrame 创建一个数据帧
func NewDataFrame(data []byte) *DataFrame {
	b := make([]byte, 1)
	_, _ = rand.Read(b)
	return &DataFrame{
		Length: uint32(len(data)),
		Obs:    b[0],
		Data:   data,
	}
}

// MarshalBinary 写入帧数据，生成字节数组
func (d *DataFrame) MarshalBinary() []byte {
	result := make([]byte, 4, 4+1+d.Length)
	binary.BigEndian.PutUint32(result, d.Length)
	result = append(result, d.Obs)
	result = append(result, d.Data...)
	for i := 5; i < len(result); i++ {
		result[i] = result[i] ^ d.Obs // 将数据进行异或混淆
	}
	return result
}

// ReadFrame 读取帧数据，返回DataFrame结构体指针
func ReadFrame(r io.Reader) (*DataFrame, error) {
	var bs [4]byte
	// read xor and magic number
	_, err := io.ReadFull(r, bs[:])
	if err != nil {
		return nil, err
	}
	fr := &DataFrame{}

	fr.Length = binary.BigEndian.Uint32(bs[:])
	// 哦不不，32M太大咯
	if fr.Length > 1024*1024*32 {
		return nil, fmt.Errorf("frame is too big, %d", fr.Length)
	}
	// 读取混淆因子
	n, err := r.Read(bs[:1])
	if n != 1 || err != nil {
		return nil, fmt.Errorf("read type error %v", err)
	}
	fr.Obs = bs[0]
	// 读取数据
	buf := make([]byte, fr.Length)
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return nil, fmt.Errorf("read data error: %v", err)
	}
	for i := 0; i < len(buf); i++ {
		buf[i] = buf[i] ^ fr.Obs
	}
	fr.Data = buf
	return fr, nil
}
