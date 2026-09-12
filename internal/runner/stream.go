package runner

import (
	"encoding/binary"
	"io"
)

// DemuxStream separates multiplexed Docker stdout and stderr streams.
func DemuxStream(src io.Reader, stdout, stderr io.Writer) error {
	header := make([]byte, 8)

	for {
		_, err := io.ReadFull(src, header)
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				return nil
			}
			return err
		}

		streamType := header[0]
		size := binary.BigEndian.Uint32(header[4:8])

		var dst io.Writer
		switch streamType {
		case 1:
			dst = stdout
		case 2:
			dst = stderr
		default:
			dst = stdout
		}

		if size > 0 {
			if _, err := io.CopyN(dst, src, int64(size)); err != nil {
				if err == io.EOF {
					return nil
				}
				return err
			}
		}
	}
}
