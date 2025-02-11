package main

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"sync"
)

type node struct {
	val  int
	next *node
}

type Queue struct {
	head  *node
	tail  *node
	size  int
	mutex sync.Mutex
}

func (q *Queue) IsEmpty() bool {
	return q.size == 0
}

func (q *Queue) Enqueue(item int) {
	q.mutex.Lock()
	defer q.mutex.Unlock()
	n := &node{item, nil}
	if q.size == 0 {
		q.head = n
		q.tail = n
	} else {
		q.tail.next = n
		q.tail = n
	}
	q.size++
}

func (q *Queue) Dequeue() (int, bool) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if q.IsEmpty() {
		return 0, false
	}

	n := q.head
	if q.size == 1 {
		q.head = nil
		q.tail = nil
	} else {
		q.head = q.head.next
		n.next = nil
	}

	q.size--
	return n.val, true
}

var (
	annouce        string
	length         int
	name           string
	pieceLength    int
	pieceHashesStr string
	infoHash       []byte
	isMagenet      bool = false
	peersList      []string
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
		pieceHashesList := pieceHashes(pieceHashesStr, length, pieceLength)
		pieceCount := int(math.Ceil(float64(length) / float64(pieceLength)))
		data, _ := downloadPiece(peersList, pieceId, pieceCount, pieceHashesList[pieceId])
		err = os.WriteFile(os.Args[3], data, 0644)
		if err != nil {
			fmt.Println(err)
		}
	} else if command == "download" {

		if err := fill(os.Args[4]); err != nil {
			fmt.Println(err)
			return
		}

		peersList, err := peers()
		if err != nil {
			fmt.Println(err)
			return
		}

		download(peersList)
	} else if command == "magnet_parse" {
		tracker, magnetInfo := parseMagnet(os.Args[2])
		fmt.Printf("Tracker URL: %s\nInfo Hash: %s\n", tracker, magnetInfo)
	} else if command == "magnet_handshake" {
		conn, _, _ := magnetHandshake(os.Args[2])
		if conn != nil {
			conn.Close()
		}

	} else if command == "magnet_info" {
		magnetInfo(os.Args[2])
	} else if command == "magnet_download_piece" {
		err := magnetInfo(os.Args[4])
		if err != nil {
			fmt.Println(err)
			return
		}
		isMagenet = false

		pieceId, _ := strconv.Atoi(os.Args[5])
		pieceHashesList := pieceHashes(pieceHashesStr, length, pieceLength)
		pieceCount := int(math.Ceil(float64(length) / float64(pieceLength)))
		data, _ := downloadPiece(peersList, pieceId, pieceCount, pieceHashesList[pieceId])
		err = os.WriteFile(os.Args[3], data, 0644)
		if err != nil {
			fmt.Println(err)
		}
	} else if command == "magnet_download" {
		err := magnetInfo(os.Args[4])
		if err != nil {
			fmt.Println(err)
			return
		}
		isMagenet = false
		download(peersList)
	} else {
		fmt.Println("Unknown command: " + command)
		os.Exit(1)
	}
}
