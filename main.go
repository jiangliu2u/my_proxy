package main

import "my_socks5/network"

func main()  {
	server :=network.NewServer()
	server.Run()
}
