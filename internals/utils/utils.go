package utils

import (
	"fmt"
	"io"
	"strings"
)

func ReadFromObject(source io.Reader, strBuilder *strings.Builder, fileSize int64) error {
	const fileSizeThreshold = 10 * 1024 * 1024 // 10MB

	// read decompressed data
	if fileSize < fileSizeThreshold { // if file size less than 10MB read all at once
		data, err := io.ReadAll(source)
		if err != nil {
			return fmt.Errorf("error reading decompressed data: %w", err)
		}
		strBuilder.Write(data)
	} else { // if file size larger than 10MB read in chunks
		buf := make([]byte, 4096) // 4KB chunk size
		for {
			n, err := source.Read(buf)
			if n > 0 {
				strBuilder.Write(buf[:n])
			}
			if err != nil {
				if err == io.EOF {
					break // end of file
				}
				return fmt.Errorf("error reading decompressed data: %s\n", err)
			}
		}
	}

	return nil
}

func SplitAtNull(str string) string {
	for i, c := range str {
		if c == 0 {
			return str[i+1:]
		}
	}
	return str
}
