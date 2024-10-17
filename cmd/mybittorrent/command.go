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
	"net/url"
	"os"
	"strings"
	"sync"
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
	data := make([][]byte, pieceCount)

	var wg sync.WaitGroup
	pieceQueue := Queue{make([]int, 0, pieceCount), sync.Mutex{}}
	for i := 0; i < pieceCount; i++ {
		pieceQueue.Enqueue(i)
	}

	for _, peerStr := range peersList {
		wg.Add(1)
		go func(peer string) {
			defer wg.Done()
			conn, err := getUnchokedPeer(peer)
			if err != nil {
				return
			}

			for !pieceQueue.IsEmpty() {
				i, _ := pieceQueue.Dequeue()
				pieceSize := pieceLength
				if i == pieceCount-1 && length%pieceLength != 0 {
					pieceSize = length % pieceLength
				}
				chunck, err := getPieceData(conn, pieceSize, i, pieceHashesList[i])
				if err != nil {
					fmt.Println(err)
					pieceQueue.Enqueue(i)
					return
				}
				data[i] = chunck
			}

		}(peerStr)
	}

	wg.Wait()

	finalData := make([]byte, 0, length)
	for _, chunk := range data {
		finalData = append(finalData, chunk...)
	}

	err = os.WriteFile(os.Args[3], finalData, 0644)
	if err != nil {
		fmt.Println("Error writing file:", err)
	}
}

func getUnchokedPeer(peer string) (net.Conn, error) {
	conn, _, err := handshake(peer)
	if err != nil {
		return nil, err
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

func downloadPiece(peerList []string, pieceId, pieceCount int, actualPieceHash string) ([]byte, error) {

	conn, err := getUnchokedPeer(peerList[0])
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

// handshake establishes a connection with the specified peer.
// It returns the connection, a boolean indicating whether extensions are supported,
// and any error encountered during the handshake process.
func handshake(peer string) (net.Conn, bool, error) {
	fmt.Println("handshaking")
	message := make([]byte, 0, 68)
	message = append(message, byte(19))
	message = append(message, []byte("BitTorrent protocol")...)
	reservedBytes := make([]byte, 8)
	if isMagenet {
		reservedBytes[5] = 16
	}
	message = append(message, reservedBytes...)
	message = append(message, infoHash...)
	message = append(message, []byte("00112233445566778899")...)

	conn, err := net.Dial("tcp", peer)
	if err != nil {
		return nil, false, err
	}

	_, err = conn.Write(message)
	if err != nil {
		fmt.Println(err)
		return nil, false, err
	}
	buf := make([]byte, 68)
	_, err = io.ReadFull(conn, buf)
	if err != nil {
		fmt.Println(err)
		return nil, false, err
	}
	fmt.Println("Peer ID:", hex.EncodeToString(buf[48:]))

	return conn, buf[25] == 16, nil
}

func magnetHandshake() {
	params, _ := url.ParseQuery(os.Args[2][8:])
	annouce = params["tr"][0]
	length = 999
	infoHash, _ = hex.DecodeString(params["xt"][0][9:])
	isMagenet = true

	peersList, err := peers()
	if err != nil {
		fmt.Println(err)
		return
	}

	conn, extensionSupport, err := handshake(peersList[0])
	if err == nil {
		defer conn.Close()
	}

	if !extensionSupport {
		return
	}

	buffer := make([]byte, 4)
	_, err = conn.Read(buffer)
	if err != nil {
		fmt.Println(err)
		return
	}

	payloadLength := binary.BigEndian.Uint32(buffer)
	payload := make([]byte, payloadLength)
	_, err = conn.Read(payload)
	if err != nil {
		fmt.Println(err)
		return
	}

	if payload[0] != 5 {
		fmt.Println("Expected bitfield message, got ", payload[0])
		return
	}

	extensionDict := []byte("d1:md11:ut_metadatai16eee")
	messageLength := len(extensionDict) + 2
	message := make([]byte, 4)
	binary.BigEndian.PutUint32(message, uint32(messageLength))
	message = append(message, 20)
	message = append(message, 0)
	message = append(message, extensionDict...)

	_, err = conn.Write(message)
	if err != nil {
		fmt.Println(err)
		return
	}

	buffer = make([]byte, 4)
	_, err = conn.Read(buffer)
	if err != nil {
		fmt.Println(err)
		return
	}

	payloadLength = binary.BigEndian.Uint32(buffer)
	payload = make([]byte, payloadLength)
	_, err = io.ReadFull(conn, payload)
	if err != nil {
		fmt.Println(err)
		return
	}

	bencodededDict, err := decodeBencode(string(payload[2:]))
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("Peer Metadata Extension ID: %d\n", bencodededDict.(map[string]interface{})["m"].(map[string]interface{})["ut_metadata"].(int))
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
	fmt.Println(decodedBody)
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
