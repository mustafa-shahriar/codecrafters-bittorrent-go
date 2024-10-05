package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
)

func CreateBitfieldMessage(payload []byte, messageId int) []byte {
	var buffer bytes.Buffer

	length := uint32(len(payload) + 1)
	binary.Write(&buffer, binary.BigEndian, length)

	messageID := byte(messageId)
	buffer.WriteByte(messageID)

	buffer.Write(payload)

	return buffer.Bytes()
}

func pieceHashes(sha string, pieceLength int) []string {
	pieces := make([]string, 0, pieceLength)
	for i := 0; i < len(sha)-19; i += 20 {
		pieceSha := sha[i : i+20]
		pieces = append(pieces, hex.EncodeToString([]byte(pieceSha)))
	}
	return pieces
}

func fill(fileName string) error {
	fileContent, err := os.ReadFile(fileName)
	if err != nil {
		fmt.Println(err)
		return err
	}

	decodedBencode, err := decodeBencode(string(fileContent))
	if err != nil {
		fmt.Println(err)
		return err
	}

	dict := decodedBencode.(map[string]interface{})
	info := dict["info"].(map[string]interface{})
	annouce = dict["announce"].(string)
	length = info["length"].(int)
	pieceLength = info["piece length"].(int)
	pieceHashesStr = info["pieces"].(string)

	infoBytes := bytes.Split(fileContent, []byte("info"))[1]
	infoBytes = infoBytes[:len(infoBytes)-1]
	hasher := sha1.New()
	hasher.Write(infoBytes)
	infoHash = hasher.Sum(nil)

	return nil
}
