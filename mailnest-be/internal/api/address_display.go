package api

import (
	"io"
	stdmime "mime"
	netmail "net/mail"
	"strings"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/transform"
)

// decodeAddressDisplayValue 只处理对前端展示的地址文本，不改动数据库原始字段。
func decodeAddressDisplayValue(value string) string {
	value = decodeMIMEHeaderDisplay(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	if address, err := netmail.ParseAddress(value); err == nil {
		return displayParsedAddress(address)
	}
	return value
}

func decodeAddressDisplayList(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = decodeAddressDisplayValue(value)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}

func splitDecodedAddressField(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return []string{}
	}
	decoded := decodeMIMEHeaderDisplay(value)
	if addresses, err := netmail.ParseAddressList(decoded); err == nil {
		result := make([]string, 0, len(addresses))
		for _, address := range addresses {
			if display := displayParsedAddress(address); display != "" {
				result = append(result, display)
			}
		}
		return result
	}
	parts := strings.FieldsFunc(decoded, func(r rune) bool {
		return r == ',' || r == ';' || r == '，' || r == '；'
	})
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if display := decodeAddressDisplayValue(part); display != "" {
			result = append(result, display)
		}
	}
	return result
}

func displayParsedAddress(address *netmail.Address) string {
	if address == nil || strings.TrimSpace(address.Address) == "" {
		return ""
	}
	name := strings.TrimSpace(decodeMIMEHeaderDisplay(address.Name))
	email := strings.TrimSpace(address.Address)
	if name == "" || strings.EqualFold(name, email) {
		return email
	}
	return name + " <" + email + ">"
}

func decodeMIMEHeaderDisplay(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	decoded, err := displayWordDecoder().DecodeHeader(value)
	if err != nil {
		return value
	}
	return strings.TrimSpace(decoded)
}

func displayWordDecoder() *stdmime.WordDecoder {
	return &stdmime.WordDecoder{CharsetReader: displayCharsetReader}
}

func displayCharsetReader(charset string, input io.Reader) (io.Reader, error) {
	decoder, err := displayCharsetDecoder(charset)
	if err != nil {
		return nil, err
	}
	return transform.NewReader(input, decoder.NewDecoder()), nil
}

func displayCharsetDecoder(charset string) (encoding.Encoding, error) {
	switch strings.ToLower(strings.TrimSpace(charset)) {
	case "", "utf-8", "utf8", "us-ascii", "ascii":
		return encoding.Nop, nil
	case "gbk", "cp936", "ms936", "windows-936", "gb2312", "gb_2312", "euc-cn":
		return simplifiedchinese.GBK, nil
	case "gb18030":
		return simplifiedchinese.GB18030, nil
	case "hz-gb-2312", "hzgb2312":
		return simplifiedchinese.HZGB2312, nil
	case "big5", "big-5", "big5-hkscs":
		return traditionalchinese.Big5, nil
	case "shift_jis", "shift-jis", "sjis", "cp932", "windows-31j":
		return japanese.ShiftJIS, nil
	case "euc-jp":
		return japanese.EUCJP, nil
	case "iso-2022-jp":
		return japanese.ISO2022JP, nil
	case "euc-kr", "ks_c_5601-1987", "ks_c_5601":
		return korean.EUCKR, nil
	case "iso-8859-1", "latin1", "latin-1":
		return encoding.Nop, nil
	default:
		return encoding.Nop, nil
	}
}
