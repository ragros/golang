package protorpc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"reflect"
	"strconv"
	"unsafe"

	pb "github.com/ragros/golang/protorpc/internal"

	"github.com/golang/protobuf/proto"
	"github.com/twinj/uuid"
)

const (
	Result_OK                int32 = 0
	Result_UNKNOWN_ERROR     int32 = 1
	Result_TIMEOUT           int32 = 2
	Result_QUEUE_FULL        int32 = 3
	Result_DUPLICATE_REQID   int32 = 4
	Result_CLIENT_EXCEPTION  int32 = 5
	Result_CLIENT_INTERRUPT  int32 = 6
	Result_SERVER_EXCEPTION  int32 = 7
	Result_LINK_BROKEN       int32 = 8
	Result_HANDLER_NOT_FOUND int32 = 9
	Result_INVALID_REQUEST   int32 = 10
)

var (
	Debug         = false
	Logger        = ILogger(log.New(os.Stderr, "[protorpc] ", log.Ltime|log.Lmicroseconds|log.Lshortfile))
	ErrLinkBroken = errors.New("BrokenLink")
	ErrNotFound   = errors.New("NotFound")
	result_name   = map[int32]string{
		0:  "OK",
		1:  "UNKNOWN_ERROR",
		2:  "TIMEOUT",
		3:  "QUEUE_FULL",
		4:  "DUPLICATE_REQID",
		5:  "CLIENT_EXCEPTION",
		6:  "CLIENT_INTERRUPT",
		7:  "SERVER_EXCEPTION",
		8:  "LINK_BROKEN",
		9:  "HANDLER_NOT_FOUND",
		10: "INVALID_REQUEST",
	}
)

type ILogger interface {
	Output(depth int, msg string) error
}

func errPrint(v ...interface{}) {
	if Logger != nil {
		Logger.Output(3, "[ERROR]"+fmt.Sprint(v...))
	}
}

func warnPrint(v ...interface{}) {
	if Logger != nil {
		Logger.Output(3, "[WARN ]"+fmt.Sprint(v...))
	}
}

func infoPrint(v ...interface{}) {
	if Logger != nil {
		Logger.Output(3, "[INFO ]"+fmt.Sprint(v...))
	}
}

func dbgPrint(v ...interface{}) {
	if Debug && Logger != nil {
		Logger.Output(3, "[DEBUG]"+fmt.Sprint(v...))
	}
}

type Handler interface {
	Handle(*Channel, *Request) *Response
}

type HandlerFunc func(*Channel, *Request) *Response

func (f HandlerFunc) Handle(c *Channel, r *Request) *Response {
	return f(c, r)
}

/**
连接建立后,优先发送数据的一方,可以在OnConnected中发送请求。
另一方可在OnConnecting时保存连接信息方便数据来临时找到对应的通道。
(通常服务器在客户端发送身份信息后将身份信息绑定到对应的通道;
而client一般只与服务器保持一个连接,可以忽略此信号)
*/

type ChannelListener interface {
	OnDisconnect(*Channel)
	OnConnecting(*Channel) //连接正在建立,此时channel尚不可用
	OnConnected(*Channel)  //连接已建立,可以通过channel发送消息
}

type dataProvider interface {
	entity(key string) *pb.Entity
	appendEntity(v *pb.Entity)
}

type dataOper struct {
	dp dataProvider
}

type Request struct {
	dataOper
	msg *pb.Request
}

func NewRequest(cmd string) *Request {
	msg := &pb.Request{
		ReqId: proto.String(uuid.Formatter(uuid.NewV4(), uuid.Clean)),
		Cmd:   &cmd,
	}
	r := &Request{
		msg: msg,
	}
	r.dp = r
	return r
}

func NewRequest2(reqId, cmd string) *Request {
	msg := &pb.Request{
		ReqId: &reqId,
		Cmd:   &cmd,
	}
	r := &Request{
		msg: msg,
	}
	r.dp = r
	return r
}

func (t *Request) entity(key string) *pb.Entity {
	for _, v := range t.msg.Entity {
		if v.GetKey() == key {
			return v
		}
	}
	return nil
}

func (t *Request) appendEntity(v *pb.Entity) {
	t.msg.Entity = append(t.msg.Entity, v)
}

func (t *Request) String() string {
	return proto.CompactTextString(t.msg)
}

func (t *Request) ReqId() string {
	return t.msg.GetReqId()
}

func (t *Request) Cmd() string {
	return t.msg.GetCmd()
}

func (t *dataOper) SetString(key string, value string) {
	en := t.dp.entity(key)
	if en == nil {
		en = &pb.Entity{
			Key:         &key,
			StringValue: &value,
		}
		t.dp.appendEntity(en)
	} else {
		en.StringValue = &value
	}

}

func (t *dataOper) SetInt64(key string, value int64) {
	en := t.dp.entity(key)
	if en == nil {
		en = &pb.Entity{
			Key:        &key,
			Int64Value: &value,
		}
		t.dp.appendEntity(en)
	} else {
		en.Int64Value = &value
	}

}

func (t *dataOper) SetInt32(key string, value int32) {
	t.SetInt64(key, int64(value))
}

func (t *dataOper) SetUint32(key string, value uint32) {
	t.SetInt64(key, int64(value))
}

func (t *dataOper) SetUint64(key string, value uint64) {
	t.SetInt64(key, int64(value))
}

func (t *dataOper) SetBool(key string, value bool) {
	en := t.dp.entity(key)
	if en == nil {
		en = &pb.Entity{
			Key:       &key,
			BoolValue: &value,
		}
		t.dp.appendEntity(en)
	} else {
		en.BoolValue = &value
	}

}

func (t *dataOper) SetJsonObject(key string, in interface{}) error {
	bs, err := json.Marshal(in)
	if err != nil {
		return err
	}
	en := t.dp.entity(key)
	if en != nil {
		en.BytesValue = bs
	} else {
		en = &pb.Entity{
			Key:        &key,
			BytesValue: bs,
		}
		t.dp.appendEntity(en)
	}
	return nil
}

func (t *dataOper) SetBytes(key string, value []byte) {
	bs := make([]byte, len(value))
	copy(bs, value)
	en := t.dp.entity(key)
	if en != nil {
		en.BytesValue = value
	} else {
		en = &pb.Entity{
			Key:        &key,
			BytesValue: value,
		}
		t.dp.appendEntity(en)
	}
}
func (t *dataOper) SetStringList(key string, value []string) {
	lst := make([]string, len(value))
	copy(lst, value)
	en := t.dp.entity(key)
	if en == nil {
		en = &pb.Entity{
			Key:        &key,
			StringList: lst,
		}
		t.dp.appendEntity(en)
	} else {
		en.StringList = lst
	}
}

func (t *dataOper) SetInt64List(key string, value []int64) {
	lst := make([]int64, len(value))
	copy(lst, value)
	en := t.dp.entity(key)
	if en == nil {
		en = &pb.Entity{
			Key:       &key,
			Int64List: lst,
		}
		t.dp.appendEntity(en)
	} else {
		en.Int64List = lst
	}
}

func (t *dataOper) SetUint64List(key string, value []uint64) {
	h := (*reflect.SliceHeader)(unsafe.Pointer(&value))
	dest := *(*[]int64)(unsafe.Pointer(h))
	t.SetInt64List(key, dest)
}

func (t *dataOper) SetUint32List(key string, value []uint32) {
	var dest []int64
	for _, v := range value {
		dest = append(dest, int64(v))
	}
	en := t.dp.entity(key)
	if en == nil {
		en = &pb.Entity{
			Key:       &key,
			Int64List: dest,
		}
		t.dp.appendEntity(en)
	} else {
		en.Int64List = dest
	}
}

func (t *dataOper) SetInt32List(key string, value []int32) {
	var dest []int64
	for _, v := range value {
		dest = append(dest, int64(v))
	}
	en := t.dp.entity(key)
	if en == nil {
		en = &pb.Entity{
			Key:       &key,
			Int64List: dest,
		}
		t.dp.appendEntity(en)
	} else {
		en.Int64List = dest
	}
}

func (t *dataOper) GetInt64(key string) (int64, error) {
	en := t.dp.entity(key)
	if en == nil {
		return 0, ErrNotFound
	}
	return en.GetInt64Value(), nil
}

func (t *dataOper) GetUint64(key string) (uint64, error) {
	v, err := t.GetInt64(key)
	return uint64(v), err
}

func (t *dataOper) GetUint32(key string) (uint32, error) {
	v, err := t.GetInt64(key)
	return uint32(v), err
}

func (t *dataOper) GetInt32(key string) (int32, error) {
	v, err := t.GetInt64(key)
	return int32(v), err
}

func (t *dataOper) GetString(key string) (string, error) {
	en := t.dp.entity(key)
	if en == nil {
		return "", ErrNotFound
	}
	return en.GetStringValue(), nil
}

func (t *dataOper) GetStringList(key string) ([]string, error) {
	en := t.dp.entity(key)
	if en == nil {
		return nil, ErrNotFound
	}
	return en.GetStringList(), nil
}

func (t *dataOper) GetInt64List(key string) ([]int64, error) {
	en := t.dp.entity(key)
	if en == nil {
		return nil, ErrNotFound
	}
	return en.GetInt64List(), nil
}

func (t *dataOper) GetUint64List(key string) ([]uint64, error) {
	en := t.dp.entity(key)
	if en == nil {
		return nil, ErrNotFound
	}
	raw := en.GetInt64List()
	if len(raw) == 0 {
		return nil, nil
	}
	h := (*reflect.SliceHeader)(unsafe.Pointer(&raw))
	dest := *(*[]uint64)(unsafe.Pointer(h))
	return dest, nil
}

func (t *dataOper) GetUint32List(key string) ([]uint32, error) {
	en := t.dp.entity(key)
	if en == nil {
		return nil, ErrNotFound
	}
	raw := en.GetInt64List()
	var dest []uint32
	for _, v := range raw {
		dest = append(dest, uint32(v))
	}
	return dest, nil
}

func (t *dataOper) GetInt32List(key string) ([]int32, error) {
	en := t.dp.entity(key)
	if en == nil {
		return nil, ErrNotFound
	}
	raw := en.GetInt64List()
	var dest []int32
	for _, v := range raw {
		dest = append(dest, int32(v))
	}
	return dest, nil
}

func (t *dataOper) GetBool(key string) (bool, error) {
	en := t.dp.entity(key)
	if en == nil {
		return false, ErrNotFound
	}
	return en.GetBoolValue(), nil
}

func (t *dataOper) GetBytes(key string) ([]byte, error) {
	en := t.dp.entity(key)
	if en == nil {
		return nil, ErrNotFound
	}
	return en.GetBytesValue(), nil
}

func (t *dataOper) GetJsonObject(key string, out interface{}) error {
	en := t.dp.entity(key)
	if en == nil {
		return ErrNotFound
	}
	bs := bytes.NewBuffer(en.GetBytesValue())
	js := json.NewDecoder(bs)
	js.UseNumber()
	return js.Decode(out)
}

func (t *Request) marshal() ([]byte, error) {
	en := &pb.Message{
		Request: t.msg,
	}
	return proto.Marshal(en)
}

type Response struct {
	dataOper
	msg *pb.Response
}

func (t *Response) String() string {
	result := t.msg.GetResult()
	errmsg := t.msg.GetErrmsg()
	if result != Result_OK {
		if desc, ok := result_name[result]; ok {
			return desc + ":" + errmsg
		}
		return strconv.FormatInt(int64(result), 10) + ":" + errmsg
	}
	return proto.CompactTextString(t.msg)
}

func NewResponse(reqId string, result int32) *Response {
	msg := &pb.Response{
		ReqId:  &reqId,
		Result: &result,
	}
	r := &Response{
		msg: msg,
	}
	r.dp = r
	return r
}

func NewResponse2(reqId string, result int32, errmsg string) *Response {
	msg := &pb.Response{
		ReqId:  &reqId,
		Result: &result,
		Errmsg: &errmsg,
	}
	r := &Response{
		msg: msg,
	}
	r.dp = r
	return r
}

func (t *Response) entity(key string) *pb.Entity {
	for _, v := range t.msg.Entity {
		if v.GetKey() == key {
			return v
		}
	}
	return nil
}

func (t *Response) appendEntity(v *pb.Entity) {
	t.msg.Entity = append(t.msg.Entity, v)
}

func (t *Response) ReqId() string {
	return t.msg.GetReqId()
}

func (t *Response) Result() int32 {
	return t.msg.GetResult()
}

func (t *Response) IsOK() bool {
	return Result_OK == t.msg.GetResult()
}

func (t *Response) SetErrMsg(value string) {
	t.msg.Errmsg = &value
}

func (t *Response) GetErrMsg() string {
	return t.msg.GetErrmsg()
}

func (t *Response) marshal() ([]byte, error) {
	en := &pb.Message{
		Response: t.msg,
	}
	return proto.Marshal(en)
}
