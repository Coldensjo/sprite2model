//go:build js && wasm

// Command wasm is the converter for the web page: built with GOOS=js GOARCH=wasm into
// web/sprite2model.wasm, it sets the global sprite2model.convert(bytes, optionsJSON, name), which
// returns spritemodel.Run's result as JSON ({"error": ...} when it fails).
package main

import (
	"encoding/json"
	"syscall/js"

	"github.com/Coldensjo/sprite2model/spritemodel"
)

func main() {
	js.Global().Set("sprite2model", js.ValueOf(map[string]any{
		"convert": js.FuncOf(convert),
		"version": spritemodel.Version,
	}))
	select {}
}

func convert(_ js.Value, args []js.Value) any {
	fail := func(msg string) any {
		b, _ := json.Marshal(map[string]string{"error": msg})
		return string(b)
	}
	if len(args) < 3 {
		return fail("convert(bytes, options, name)")
	}
	data := make([]byte, args[0].Get("length").Int())
	js.CopyBytesToGo(data, args[0])
	var opt spritemodel.Options
	if err := json.Unmarshal([]byte(args[1].String()), &opt); err != nil {
		return fail("options: " + err.Error())
	}
	res, err := spritemodel.Run(args[2].String(), data, opt)
	if err != nil {
		return fail(err.Error())
	}
	b, err := json.Marshal(res)
	if err != nil {
		return fail(err.Error())
	}
	return string(b)
}
