package main

import (
	"fmt"
	"math"
	"net/url"
	"os"
	"strconv"
	"sync"
)

type Queue struct {
	items []int
	mutex sync.Mutex
}

func (q *Queue) IsEmpty() bool {
	return len(q.items) == 0
}

func (q *Queue) Enqueue(item int) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	q.items = append(q.items, item)
}

func (q *Queue) Dequeue() (int, bool) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	if len(q.items) == 0 {
		return 0, false
	}
	item := q.items[0]
	q.items = q.items[1:]
	return item, true
}

var (
	annouce        string
	length         int
	name           string
	pieceLength    int
	pieceHashesStr string
	infoHash       []byte
	isMagenet      bool = false
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

		if conn, _, err := handshake(os.Args[3]); err != nil {
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
		pieceCount := int(math.Ceil(float64(length) / float64(pieceLength)))
		data, _ := downloadPiece(peersList, pieceId, pieceCount, pieceHashesList[pieceId])
		err = os.WriteFile(os.Args[3], data, 0644)
		if err != nil {
			fmt.Println(err)
		}
	} else if command == "download" {
		download()
	} else if command == "magnet_parse" {
		params, _ := url.ParseQuery(os.Args[2][8:])
		fmt.Printf("Tracker URL: %s\nInfo Hash: %s\n", params["tr"][0], params["xt"][0][9:])
	} else if command == "magnet_handshake" {
		magnetHandshake()

	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
