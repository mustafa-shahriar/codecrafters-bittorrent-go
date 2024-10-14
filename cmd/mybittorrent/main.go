package main

import (
	"fmt"
	"os"
	"strconv"
)

var (
	annouce        string
	length         int
	name           string
	pieceLength    int
	pieceHashesStr string
	infoHash       []byte
)

func main() {

	command := os.Args[1]

	if command == "decode" {
		decode()
	} else if command == "info" {
		info()
	} else if command == "peers" {
		if err := fill(os.Args[2]); err != nil {
			fmt.Println(err)
			return
		}
		peerList, err := peers()
		if err != nil {
			fmt.Println(err)
			return
		}
		for _, peer := range peerList {
			fmt.Println(peer)
		}
	} else if command == "handshake" {
		if err := fill(os.Args[2]); err != nil {
			fmt.Println(err)
			return
		}

		if conn, err := handshake(os.Args[3]); err != nil {
			conn.Close()
		}
	} else if command == "download_piece" {
		if err := fill(os.Args[4]); err != nil {
			fmt.Println(err)
			return
		}

		peersList, err := peers()
		if err != nil {
			fmt.Println(err)
			return
		}
		pieceId, _ := strconv.Atoi(os.Args[5])
		pieceHashesList := pieceHashes(pieceHashesStr, length)
		data, _ := downloadPiece(peersList, pieceId, 0, pieceHashesList[pieceId])
		err = os.WriteFile(os.Args[3], data, 0644)
		if err != nil {
			fmt.Println(err)
		}
	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
