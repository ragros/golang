# protorpc

1.允许一个KEY同时包含Bool 、String、整型、[]String,[]整型,[]byte中任意组合。
整型用一个公共空间存储，因此int32、uint32、int64、uint64不能共存([]整型也是)。反复设值将覆盖。
 例：

	
	req.SetString("key1","stringvalue")
	req.SetBool("key1",true)
	req.SetBytes("key1",[]byte("bytes value"))
	req.SetStringList("key1",[]string{"hello","world"})
	req.SetInt32List("key1",[]int32{100,-200})
	req.SetInt64List("key1",[]int64{-1,300})
	req.SetInt64("key1",-1)
	req.SetUint64("key1",2)
	req.SetUint32("key1",3)	
	req.SetInt32("key1",-4)

	对端取值:
	req.GetString("key1")      stringvalue
	req.GetBool("key1")        true
	req.GetStringList("key1")  [hello world]
	req.GetUint32List("key1")  0xffffffff,300  (最后一次设置有效)   
	req.GetUint64("key1")      0xFFFFFFFFFFFFFFFC 
	req.GetInt64("key1")       -4
	req.GetUint32("key1")	   0xFFFFFFFC 
	req.GetInt32("key1")       -4


3.协议使用int64作为整型的存储空间(其实uint64也是一样)。
  考虑这样做的原因：1.同key包含多个数据的几乎没人用到。2.方便通信双端的类型适配。



