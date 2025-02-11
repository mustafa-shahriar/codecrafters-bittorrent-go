package main

import (
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

func download(peersList []string) {

	pieceHashesList := pieceHashes(pieceHashesStr, length, pieceLength)
	pieceCount := int(math.Ceil(float64(length) / float64(pieceLength)))
	data := make([][]byte, pieceCount)

	var wg sync.WaitGroup
	pieceQueue := Queue{mutex: sync.Mutex{}}
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
			defer conn.Close()

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

	err := os.WriteFile(os.Args[3], finalData, 0644)
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
		conn.Close()
		fmt.Println(err)
		return nil, err
	}

	payloadLength := binary.BigEndian.Uint32(buffer)
	payload := make([]byte, payloadLength)
	_, err = conn.Read(payload)
	if err != nil {
		conn.Close()
		fmt.Println(err)
		return nil, err
	}

	if payload[0] != 5 {
		conn.Close()
		fmt.Println("Expected bitfield message, got ", payload[0])
		return nil, err
	}

	_, err = conn.Write([]byte{0x00, 0x00, 0x00, 0x01, 0x02})
	if err != nil {
		conn.Close()
		fmt.Println(err)
		return nil, err
	}

	buffer = make([]byte, 4)
	_, err = conn.Read(buffer)
	if err != nil {
		conn.Close()
		fmt.Println(err)
		return nil, err
	}

	payload = make([]byte, binary.BigEndian.Uint32(buffer))
	_, err = conn.Read(payload)
	if err != nil {
		conn.Close()
		fmt.Println(err)
		return nil, err
	}

	if payload[0] != 1 {
		conn.Close()
		fmt.Println("peer didn't Unchoke")
		return nil, fmt.Errorf("peer didn't Unchoke")
	}

	return conn, nil
}

func downloadPiece(peerList []string, pieceId, pieceCount int, actualPieceHash string) ([]byte, error) {

	var conn net.Conn
	var err error
	for _, peer := range peerList {
		conn, err = getUnchokedPeer(peer)
		fmt.Println(peer)
		if err == nil {
			break
		}
	}
	if err != nil {
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

	pieceHashStr := hex.EncodeToString(hashBytes(data))

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
		conn.Close()
		return nil, false, err
	}

	_, err = conn.Write(message)
	if err != nil {
		conn.Close()
		fmt.Println(err)
		return nil, false, err
	}
	buf := make([]byte, 68)
	_, err = io.ReadFull(conn, buf)
	if err != nil {
		conn.Close()
		fmt.Println(err)
		return nil, false, err
	}
	fmt.Println("Peer ID:", hex.EncodeToString(buf[48:]))

	return conn, buf[25] == 16, nil
}

func parseMagnet(link string) (string, string) {
	params, _ := url.ParseQuery(link[8:])
	return params["tr"][0], params["xt"][0][9:]
}

func magnetInfo(link string) error {
	conn, extId, err := magnetHandshake(link)
	if err != nil {
		fmt.Println(err)
		return err
	}
	defer conn.Close()

	payload := []byte("d8:msg_typei0e5:piecei0ee")
	message := make([]byte, 4, 6+len(payload))
	binary.BigEndian.PutUint32(message[0:4], uint32(len(payload)+2))
	message = append(message, 20)
	message = append(message, byte(extId))
	message = append(message, payload...)

	_, err = conn.Write(message)
	if err != nil {
		fmt.Println(err)
		return err
	}

	buffer := make([]byte, 4)
	_, err = conn.Read(buffer)
	if err != nil {
		fmt.Println(err)
		return err
	}

	payloadLen := binary.BigEndian.Uint32(buffer)
	buffer = make([]byte, payloadLen)
	_, err = io.ReadFull(conn, buffer)
	if err != nil {
		fmt.Println(err)
		return err
	}

	fmt.Println(string(buffer[2:]))
	fmt.Println(string(buffer))

	_, n, err := decodeBencodeDict(string(buffer[2:]), 1)
	if err != nil {
		fmt.Println(err)
		return err
	}

	infoDict, err := decodeBencode(string(buffer[n+3:]))
	if err != nil {
		fmt.Println(err)
		return err
	}
	infoDictMap := infoDict.(map[string]interface{})

	infoHash = hashBytes(buffer[n+3:])
	magnetInfoHashStr := hex.EncodeToString(infoHash)
	tracker, expectedInfoHash := parseMagnet(link)
	if expectedInfoHash != magnetInfoHashStr {
		fmt.Println("Invalid Info provided by peer")
		return err
	}

	pieceLength = infoDictMap["piece length"].(int)
	length = infoDictMap["length"].(int)
	annouce = tracker
	pieceHashesStr = infoDictMap["pieces"].(string)

	fmt.Printf("Tracker URL: %s\n", tracker)
	fmt.Printf("Length: %d\n", length)
	fmt.Printf("Info Hash: %s\n", magnetInfoHashStr)
	fmt.Printf("Piece Length: %d\n", pieceLength)
	fmt.Println("Piece Hashes:")
	for _, piece := range pieceHashes(pieceHashesStr, length, pieceLength) {
		fmt.Println(piece)
	}

	return nil
}

func magnetHandshake(link string) (net.Conn, int, error) {
	params, _ := url.ParseQuery(link[8:])
	annouce = params["tr"][0]
	length = 999
	infoHash, _ = hex.DecodeString(params["xt"][0][9:])
	isMagenet = true

	peerslist, err := peers()
	if err != nil {
		fmt.Println(err)
		return nil, 0, err
	}
	peersList = peerslist

	conn, extensionSupport, err := handshake(peersList[0])

	if !extensionSupport {
		conn.Close()
		return conn, 0, fmt.Errorf("peer doesn't support extension")
	}

	buffer := make([]byte, 4)
	_, err = conn.Read(buffer)
	if err != nil {
		conn.Close()
		fmt.Println(err)
		return conn, 0, err
	}

	payloadLength := binary.BigEndian.Uint32(buffer)
	payload := make([]byte, payloadLength)
	_, err = conn.Read(payload)
	if err != nil {
		conn.Close()
		fmt.Println(err)
		return conn, 0, err
	}

	if payload[0] != 5 {
		conn.Close()
		fmt.Println("Expected bitfield message, got ", payload[0])
		return conn, 0, err
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
		conn.Close()
		fmt.Println(err)
		return conn, 0, err
	}

	buffer = make([]byte, 4)
	_, err = conn.Read(buffer)
	if err != nil {
		conn.Close()
		fmt.Println(err)
		return conn, 0, err
	}

	payloadLength = binary.BigEndian.Uint32(buffer)
	payload = make([]byte, payloadLength)
	_, err = io.ReadFull(conn, payload)
	if err != nil {
		conn.Close()
		fmt.Println(err)
		return conn, 0, err
	}

	bencodededDict, err := decodeBencode(string(payload[2:]))
	if err != nil {
		conn.Close()
		fmt.Println(err)
		return conn, 0, err
	}
	metadataExtensionID := bencodededDict.(map[string]interface{})["m"].(map[string]interface{})["ut_metadata"].(int)
	fmt.Printf("Peer Metadata Extension ID: %d\n", metadataExtensionID)

	return conn, metadataExtensionID, nil
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
	for _, piece := range pieceHashes(pieceHashesStr, length, pieceLength) {
		fmt.Println(piece)
	}

}
