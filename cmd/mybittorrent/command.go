package main

import (
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"strings"
)

func download() {
	if err := fill(os.Args[4]); err != nil {
		fmt.Println(err)
		return
	}

	peersList, err := peers()
	if err != nil {
		fmt.Println(err)
		return
	}

	pieceHashesList := pieceHashes(pieceHashesStr, length)
	pieceCount := int(math.Ceil(float64(length) / float64(pieceLength)))
	data := make([]byte, 0, length)

	conn, err := getUnchokedPeer(peersList, 0)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer conn.Close()

	for i := 0; i < pieceCount; i++ {

		pieceSize := pieceLength
		if i == pieceCount-1 && length%pieceLength != 0 {
			pieceSize = length % pieceLength
		}
		chunck, err := getPieceData(conn, pieceSize, i, pieceHashesList[i])
		if err != nil {
			fmt.Println(err)
			return
		}
		data = append(data, chunck...)
	}

	err = os.WriteFile(os.Args[3], data, 0644)
	if err != nil {
		fmt.Println(err)
	}
}

func getUnchokedPeer(peerList []string, peerIndex int) (net.Conn, error) {
	if peerIndex == len(peerList) {
		return nil, fmt.Errorf("no peer available")
	}

	conn, err := handshake(peerList[peerIndex])
	if err != nil {
		return getUnchokedPeer(peerList, peerIndex+1)
	}

	buffer := make([]byte, 4)
	_, err = conn.Read(buffer)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	payloadLength := binary.BigEndian.Uint32(buffer)
	payload := make([]byte, payloadLength)
	_, err = conn.Read(payload)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	if payload[0] != 5 {
		fmt.Println("Expected bitfield message, got ", payload[0])
		return nil, err
	}

	_, err = conn.Write([]byte{0x00, 0x00, 0x00, 0x01, 0x02})
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	buffer = make([]byte, 4)
	_, err = conn.Read(buffer)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	payload = make([]byte, binary.BigEndian.Uint32(buffer))
	_, err = conn.Read(payload)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	if payload[0] != 1 {
		fmt.Println("peer didn't Unchoke")
		return nil, err
	}

	return conn, nil
}

func downloadPiece(peerList []string, pieceId, peerIndex, pieceCount int, actualPieceHash string) ([]byte, error) {

	conn, err := getUnchokedPeer(peerList, 0)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	defer conn.Close()

	pieceSize := pieceLength
	if pieceId == pieceCount-1 && length%pieceLength != 0 {
		pieceSize = length % pieceLength
	}

	return getPieceData(conn, pieceSize, pieceId, actualPieceHash)

}

func getPieceData(conn net.Conn, pieceSize, pieceId int, actualPieceHash string) ([]byte, error) {
	blockSize := 16384
	blockCount := int(math.Ceil(float64(pieceSize) / float64(blockSize)))

	data := make([]byte, 0, pieceSize)
	for i := 0; i < blockCount; i++ {
		if i == blockCount-1 && pieceSize%blockSize != 0 {
			blockSize = pieceSize % blockSize
		}

		message := make([]byte, 17)
		binary.BigEndian.PutUint32(message[0:4], 13)
		message[4] = 6
		binary.BigEndian.PutUint32(message[5:9], uint32(pieceId))
		binary.BigEndian.PutUint32(message[9:13], uint32(i*16384))
		binary.BigEndian.PutUint32(message[13:17], uint32(blockSize))

		_, err := conn.Write(message)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}

		buffer := make([]byte, 4)
		_, err = conn.Read(buffer)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}

		payload := make([]byte, binary.BigEndian.Uint32(buffer))
		_, err = io.ReadFull(conn, payload)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}

		data = append(data, payload[9:]...)

	}

	hasher := sha1.New()
	hasher.Write(data)
	pieceHash := hasher.Sum(nil)
	pieceHashStr := hex.EncodeToString(pieceHash)

	if pieceHashStr != actualPieceHash {
		fmt.Println("piece Hash didn't match")
		return nil, fmt.Errorf("piece Hash didn't match")
	}

	return data, nil
}

func handshake(peer string) (net.Conn, error) {
	fmt.Println("handshaking")
	message := make([]byte, 0, 68)
	message = append(message, byte(19))
	message = append(message, []byte("BitTorrent protocol")...)
	message = append(message, make([]byte, 8)...)
	message = append(message, infoHash...)
	message = append(message, []byte("00112233445566778899")...)

	conn, err := net.Dial("tcp", peer)
	if err != nil {
		return nil, err
	}

	_, err = conn.Write(message)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	buf := make([]byte, 68)
	_, err = conn.Read(buf)
	if err != nil {
		fmt.Println(err)
		return nil, err
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
		peersList = append(peersList, strings.TrimSpace(peer))
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
