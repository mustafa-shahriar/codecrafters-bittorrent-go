package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
)

func downloadPiece(peerList []string, pieceId, peerIndex int) ([]byte, error) {
	if peerIndex == len(peerList) {
		return nil, errors.New("no peer available")
	}
	conn, err := handshake(peerList[peerIndex])
	if err != nil {
		return downloadPiece(peerList, pieceId, peerIndex+1)
	}
	return downloadPieceHelper(conn, pieceId), nil

}

func downloadPieceHelper(conn net.Conn, index int) []byte {
	defer conn.Close()

	// wait for the bitfield message (id = 5)
	buf := make([]byte, 4)
	_, err := conn.Read(buf)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	peerMessage := PeerMessage{}
	peerMessage.lengthPrefix = binary.BigEndian.Uint32(buf)
	payloadBuf := make([]byte, peerMessage.lengthPrefix)
	_, err = conn.Read(payloadBuf)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	peerMessage.id = payloadBuf[0]

	// fmt.Printf("Received message: %v\n", peerMessage)
	if peerMessage.id != 5 {
		fmt.Println("Expected bitfield message")
		return nil
	}

	// send interested message (id = 2)
	_, err = conn.Write([]byte{0, 0, 0, 1, 2})
	if err != nil {
		fmt.Println(err)
		return nil
	}

	// wait for unchoke message (id = 1)
	buf = make([]byte, 4)
	_, err = conn.Read(buf)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	peerMessage = PeerMessage{}
	peerMessage.lengthPrefix = binary.BigEndian.Uint32(buf)
	payloadBuf = make([]byte, peerMessage.lengthPrefix)
	_, err = conn.Read(payloadBuf)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	peerMessage.id = payloadBuf[0]

	// fmt.Printf("Received message: %v\n", peerMessage)
	if peerMessage.id != 1 {
		fmt.Println(buf)
		fmt.Println("Expected unchoke message")
		return nil
	}

	// send request message (id = 6) for each block
	// Break the piece into blocks of 16 kiB (16 * 1024 bytes) and send a request message for each block
	pieceSize := pieceLength
	pieceCnt := int(math.Ceil(float64(length) / float64(pieceSize)))
	if index == pieceCnt-1 {
		pieceSize = length % pieceLength
	}
	blockSize := 16 * 1024
	blockCnt := int(math.Ceil(float64(pieceSize) / float64(blockSize)))
	// fmt.Printf("File Length: %d, Piece Length: %d, Piece Count: %d, Block Size: %d, Block Count: %d\n", torrent.Info.Length, torrent.Info.PieceLength, pieceCnt, blockSize, blockCnt)
	var data []byte
	for i := 0; i < blockCnt; i++ {
		blockLength := blockSize
		if i == blockCnt-1 {
			blockLength = pieceSize - ((blockCnt - 1) * int(blockSize))
		}
		peerMessage := PeerMessage{
			lengthPrefix: 13,
			id:           6,
			index:        uint32(index),
			begin:        uint32(i * int(blockSize)),
			length:       uint32(blockLength),
		}

		var buf bytes.Buffer
		binary.Write(&buf, binary.BigEndian, peerMessage)
		_, err = conn.Write(buf.Bytes())
		if err != nil {
			fmt.Println(err)
			return nil
		}
		// fmt.Println("Sent request message", peerMessage)

		// wait for piece message (id = 7)
		resBuf := make([]byte, 4)
		_, err = conn.Read(resBuf)
		if err != nil {
			fmt.Println(err)
			return nil
		}

		peerMessage = PeerMessage{}
		peerMessage.lengthPrefix = binary.BigEndian.Uint32(resBuf)
		payloadBuf := make([]byte, peerMessage.lengthPrefix)
		_, err = io.ReadFull(conn, payloadBuf)
		if err != nil {
			fmt.Println(err)
			return nil
		}
		peerMessage.id = payloadBuf[0]
		// fmt.Printf("Received message: %v\n", peerMessage)

		data = append(data, payloadBuf[9:]...)
	}

	fmt.Println(data)
	return data
}

func handshake(peer string) (net.Conn, error) {
	message := make([]byte, 0, 68)
	message = append(message, byte(19))
	message = append(message, []byte("BitTorrent protocol")...)
	message = append(message, make([]byte, 8)...)
	message = append(message, infoHash...)
	message = append(message, []byte("00112233445566778899")...)

	conn, err := net.Dial("tcp", peer)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	_, err = conn.Write(message)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	buf := make([]byte, 68)
	for {
		_, err := conn.Read(buf)
		if err != nil && err == io.EOF {
			break
		}
		if err != nil {
			fmt.Println(err)
			return nil, err
		}
	}
	fmt.Println("Peer ID:", hex.EncodeToString(buf[48:]))

	return conn, nil
}

func peers() ([]string, error) {
	infoHex := hex.EncodeToString(infoHash)
	infoRaw := ""
	for i := 0; i < len(infoHex); i += 2 {
		infoRaw += "%" + infoHex[i:i+2]
	}
	url := fmt.Sprintf("%s?info_hash=%s&peer_id=00112233445566778899&port=6881&uploaded=0&downloaded=0&left=%d&compact=1", annouce, infoRaw, length)

	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("Error making GET request:", err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response body:", err)
		return nil, err
	}

	decodedBody, err := decodeBencode(string(body))
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	bodyDict := decodedBody.(map[string]interface{})
	body = []byte(bodyDict["peers"].(string))

	peersList := make([]string, 0, 5)
	for i := 0; i < len(body); i += 6 {
		ip := net.IP(body[i : i+4])
		port := binary.BigEndian.Uint16(body[i+4 : i+6])
		peer := fmt.Sprintf("%s:%d\n", ip, port)
		peersList = append(peersList, peer)
	}

	return peersList, nil
}

func decode() {
	bencodedValue := os.Args[2]

	decoded, err := decodeBencode(bencodedValue)
	if err != nil {
		fmt.Println(err)
		return
	}

	jsonOutput, _ := json.Marshal(decoded)
	fmt.Println(string(jsonOutput))
}

func info() {
	err := fill(os.Args[2])
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Printf("Tracker URL: %s\n", annouce)
	fmt.Printf("Length: %d\n", length)
	fmt.Printf("Info Hash: %s\n", hex.EncodeToString(infoHash))
	fmt.Printf("Piece Length: %d\n", pieceLength)
	fmt.Println("Piece Hashes:")
	for _, piece := range pieceHashes(pieceHashesStr, pieceLength) {
		fmt.Println(piece)
	}

}
