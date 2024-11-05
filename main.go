package main

import server "ExecutionEngine/server"

func main() {
	s := server.NewServer()
	s.Initialize()
	s.Serve()
}
