# sACN-to-OSC Bridge

This is a very simple sACN to OpenSoundControl (OSC) bridge, that receives sACN broadcast messages for a single sACN universe,
extracts the DMX value of a single channel in that universe, and sends a OSC command whenever the DMX value changes from N to N+1.

## Command-line options

  * `-sacn-iface` network interface to listen for sACN messages
  * `-sacn-universe` sACN universe (default `1`)
  * `-dmx-channel` DMX channel (default `1`)
  * `-osc-address` OSC address to send commands to (default `127.0.0.1:53000`)
  * `-osc-command` OSC command template - %d is replaced by the received DMX value (default `/cue/%d/go`)	
