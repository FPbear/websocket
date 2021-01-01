package wsSDK

import (
	"encoding/binary"
	"github.com/golang/protobuf/proto"
)

const MsgMinLength = 8

// EncodeMsg The EncodeMsg is to package messages and command
// the package format:
// | 0 | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | ... | n |
// |msg len|command|reserved field | data...     |
func encodeMsg(cmd uint16, msg proto.Message) ([]byte, error) {
	msgs, err := proto.Marshal(msg)
	if err != nil {
		return nil, err
	}
	length := MsgMinLength + len(msgs)
	data := make([]byte, length)
	copy(data[0:], uint16ToBytes(uint16(length)))
	copy(data[2:], uint16ToBytes(cmd))
	copy(data[8:], msgs)
	return data, nil
}

// uint16ToBytes
func uint16ToBytes(in uint16) []byte {
	tmp := make([]byte, 2)
	binary.LittleEndian.PutUint16(tmp, in)
	return tmp
}
