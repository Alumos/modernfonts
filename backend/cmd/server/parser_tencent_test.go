package main

import (
	"encoding/base64"
	"testing"

	"google.golang.org/protobuf/encoding/protowire"
)

func TestExtractFontItemsDropsReplacementCharactersFromFontName(t *testing.T) {
	raw := append([]byte{0xff}, []byte(" 果酱丸 5字重100/90\nHYPERLINK https://example.lanzou.com/font dkey abc123")...)
	items := extractFontItems([]string{base64.StdEncoding.EncodeToString(raw)})

	if len(items) != 1 {
		t.Fatalf("items length = %d, want 1", len(items))
	}
	if items[0].FontName != "果酱丸 5字重100/90" {
		t.Fatalf("font name = %q, want %q", items[0].FontName, "果酱丸 5字重100/90")
	}
	if items[0].AccessCode != "abc123" {
		t.Fatalf("access code = %q, want abc123", items[0].AccessCode)
	}
}

func TestExtractFontItemsReadsOnlyVisibleTencentCommandText(t *testing.T) {
	commands := []string{
		encodeTencentCommand("上一款字体\nHYPERLINK https://example.lanzou.com/previous dkey old123\n喜茶中式灵感宋L自用版 100/90/85/80/70/60", "\nJ 9 :\n微软雅黑*\n2"),
		encodeTencentCommand("\nHYPERLINK https://example.lanzou.com/current dkey new456\n万年黑 100/90/85/80/70/60\nHYPERLINK https://example.lanzou.com/next dkey next789", "\nJ : @\n2"),
	}

	items := extractFontItems(commands)
	if len(items) != 3 {
		t.Fatalf("items length = %d, want 3", len(items))
	}
	if items[1].FontName != "喜茶中式灵感宋L自用版 100/90/85/80/70/60" {
		t.Fatalf("font name = %q", items[1].FontName)
	}
	if items[1].AccessCode != "new456" {
		t.Fatalf("access code = %q, want new456", items[1].AccessCode)
	}
	if items[2].FontName != "万年黑 100/90/85/80/70/60" {
		t.Fatalf("next font name = %q", items[2].FontName)
	}
}

func TestExtractFontItemsUsesDirectPreviousVisibleLine(t *testing.T) {
	text := "2025年\n轻吟Pro 六字重\nhttps://example.lanzou.com/light\n稚\nhttps://example.lanzou.com/single\n海街圆2.0六字重\nhttps://example.lanzou.com/repeated\n海街圆六字重\nhttps://example.lanzou.com/repeated"
	items := extractFontItems([]string{encodeTencentCommand(text, "\n2\nJ : @")})

	if len(items) != 4 {
		t.Fatalf("items length = %d, want 4", len(items))
	}
	wantNames := []string{"轻吟Pro 六字重", "稚", "海街圆2.0六字重", "海街圆六字重"}
	for index, want := range wantNames {
		if items[index].FontName != want {
			t.Errorf("item %d font name = %q, want %q", index, items[index].FontName, want)
		}
	}
	if items[0].DownloadURL != "https://example.lanzou.com/light" {
		t.Fatalf("plain URL = %q", items[0].DownloadURL)
	}
}

func TestClearlyInvalidTencentFontName(t *testing.T) {
	for _, value := range []string{"2", "ڑ 2"} {
		if !isClearlyInvalidTencentFontName(value) {
			t.Errorf("isClearlyInvalidTencentFontName(%q) = false", value)
		}
	}
	for _, value := range []string{"稚", "iOS26苹方AGO", "OPPO Rounded 4.0"} {
		if isClearlyInvalidTencentFontName(value) {
			t.Errorf("isClearlyInvalidTencentFontName(%q) = true", value)
		}
	}
}

func encodeTencentCommand(text, metadata string) string {
	insert := protowire.AppendTag(nil, 1, protowire.BytesType)
	insert = protowire.AppendBytes(insert, []byte(text))
	operation := protowire.AppendTag(nil, 1, protowire.VarintType)
	operation = protowire.AppendVarint(operation, 1)
	operation = protowire.AppendTag(operation, 6, protowire.BytesType)
	operation = protowire.AppendBytes(operation, insert)
	command := protowire.AppendTag(nil, 2, protowire.BytesType)
	command = protowire.AppendBytes(command, operation)
	if metadata != "" {
		command = protowire.AppendTag(command, 3, protowire.BytesType)
		command = protowire.AppendBytes(command, []byte(metadata))
	}
	return base64.StdEncoding.EncodeToString(command)
}
