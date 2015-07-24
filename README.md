mqtt
====

MQTT Servers(Broker) in Go

For docs, see: http://godoc.org/github.com/zwczou/mqtt


**Features**

* Supports QOS 0, 1 and 2 messages
* Supports will messages
* Supports retained messages (add/remove)

**Limitations**

At this time, the following limitations apply:
 * messages are only stored in RAM
 * clean session not support
