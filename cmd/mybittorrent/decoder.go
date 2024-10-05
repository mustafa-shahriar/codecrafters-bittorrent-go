package main

import (
	"fmt"
	"strconv"
	"unicode"
)

// Example:
// - 5:hello -> hello
// - 10:hello12345 -> hello12345
func decodeBencode(bencodedString string) (interface{}, error) {
	if unicode.IsDigit(rune(bencodedString[0])) {
		str, _, err := decodeBencodeString(bencodedString, 0)
		return str, err

	} else if bencodedString[0] == 'i' {
		number, _, err := decodeBencodeInteger(bencodedString, 1)
		return number, err

	} else if bencodedString[0] == 'l' {
		list, _, err := decodeBencodeList(bencodedString, 1)
		return list, err

	} else if bencodedString[0] == 'd' {
		dict, _, err := decodeBencodeDict(bencodedString, 1)
		return dict, err

	} else {
		return nil, fmt.Errorf("Unsupported type encountered: %c", bencodedString[0])
	}
}

func decodeBencodeDict(bencodedString string, i int) (interface{}, int, error) {
	list, lastIndex, err := decodeBencodeList(bencodedString, i)
	if err != nil || len(list)%2 != 0 {
		return nil, 0, fmt.Errorf("Invalid dict")
	}

	dict := make(map[string]interface{})
	for i := 0; i < len(list)-1; i += 2 {
		key, ok := list[i].(string)
		if !ok {
			return nil, 0, fmt.Errorf("Invalid key")
		}
		dict[key] = list[i+1]
	}

	return dict, lastIndex, nil
}

func decodeBencodeList(bencodedString string, i int) ([]interface{}, int, error) {
	var list []interface{} = make([]interface{}, 0, 4)

	for i < len(bencodedString) {
		if unicode.IsDigit(rune(bencodedString[i])) {
			str, lastIndex, err := decodeBencodeString(bencodedString, i)
			if err != nil {
				return nil, 0, err
			}
			i = lastIndex
			list = append(list, str)

		} else if bencodedString[i] == 'i' {
			number, lastIndex, err := decodeBencodeInteger(bencodedString, i+1)
			if err != nil {
				return nil, 0, err
			}
			i = lastIndex + 1
			list = append(list, number)

		} else if bencodedString[i] == 'l' {
			array, lastIndex, err := decodeBencodeList(bencodedString, i+1)
			if err != nil {
				return nil, 0, err
			}
			i = lastIndex + 1
			list = append(list, array)

		} else if bencodedString[i] == 'd' {
			dict, lastIndex, err := decodeBencodeDict(bencodedString, i+1)
			if err != nil {
				return nil, 0, err
			}
			i = lastIndex + 1
			list = append(list, dict)
		} else if bencodedString[i] == 'e' {
			break

		} else {
			return nil, 0, fmt.Errorf("Unsupported type encountered: %c", bencodedString[i])
		}
	}

	return list, i, nil
}

func decodeBencodeString(bencodedString string, firstIndex int) (interface{}, int, error) {

	var firstColonIndex int

	for i := firstIndex; i < len(bencodedString); i++ {
		if bencodedString[i] == ':' {
			firstColonIndex = i
			break
		}
	}

	lengthStr := bencodedString[firstIndex:firstColonIndex]

	length, err := strconv.Atoi(lengthStr)
	if err != nil {
		return "", 0, err
	}

	return bencodedString[firstColonIndex+1 : firstColonIndex+1+length], firstColonIndex + 1 + length, nil
}

func decodeBencodeInteger(bencodedString string, firstIndex int) (interface{}, int, error) {

	var lastIndex int
	for i := firstIndex; i < len(bencodedString); i++ {
		if bencodedString[i] == 'e' {
			lastIndex = i
			break
		}
	}

	number, err := strconv.Atoi(bencodedString[firstIndex:lastIndex])
	if err != nil {
		return nil, 0, err
	}

	return number, lastIndex, nil

}
