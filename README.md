# websocket

## 目的
练习写SDK，以websocket为原型写一个SDK样式的东西。

## 数据格式
该练习，以proto作为数据传输的格式，
数据流格式：  
```
 | 0 | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | ... | n |  
 |msg len|command|reserved field | data...     |
```

## 接入
接入只需
`NewWsTask(w http.ResponseWriter, r *http.Request, conf *ServerConfig, ) (WebSocketTask, error)`，
然后在`Handle(handle func(data Message) (uint16, proto.Message))`中完成自己的逻辑即可。