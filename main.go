package main

import (
	"embed"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"

	"github.com/360EntSecGroup-Skylar/excelize"
	"github.com/zserge/lorca"
)

//go:embed www
var fs embed.FS

// Go types that are bound to the UI must be thread-safe, because each binding
// is executed in its own goroutine. In this simple case we may use atomic
// operations, but for more complex cases one should use proper synchronization.
type msg struct {
	sync.Mutex
	text string
}

func (m *msg) msg_text() string {
	m.Lock()
	defer m.Unlock()
	return m.text
}

func (m *msg) excel_diff(fileName1 string, fileName2 string, col1 int, col2 int) string {

	log.Println("excel_diff %v %v", fileName1, fileName2)
	if col1 < 0 {
		return "col1 can not less 0"
	}
	if col2 < 0 {
		return "col2 can not less 0"
	}
	fs1, err := excelize.OpenFile(fileName1)
	if err != nil {
		return fmt.Sprintf("读取文件 %v 错误, err is %+v", fileName1, err)
	}
	rows1, err := fs1.GetRows("Sheet1")
	if err != nil {
		return fmt.Sprintf("GetRows error %v, err is %+v", fileName1, err)
	}
	fs2, err := excelize.OpenFile(fileName2)
	if err != nil {
		return fmt.Sprintf("读取文件 %v 错误, err is %+v", fileName2, err)
	}
	rows2, err := fs2.GetRows("Sheet1")
	if err != nil {
		return fmt.Sprintf("GetRows error %v, err is %+v", fileName2, err)
	}

	style, _ := fs1.NewStyle(`{"border":[{"type":"left","color":"000000","style":1},{"type":"top","color":"000000","style":1},{"type":"bottom","color":"000000","style":1},{"type":"right","color":"000000","style":1}],"fill":{"type":"pattern","color":["#ffeb00"],"pattern":1},"alignment":{"horizontal":"left","ident":1,"vertical":"center","wrap_text":true}}`)

	for rindex, row1 := range rows1 {
		if (col1 >= len(row1)) {
			return fmt.Sprintf("主文件 %v 在 %v 行没有找到 %v 列", fileName1, rindex, col1)
		}
		for rindex2, row2 := range rows2 {
			if (col2 >= len(row2)) {
				return fmt.Sprintf("主文件 %v 在 %v 行没有找到 %v 列", fileName2, rindex2, col2)
			}
			if row1[col1] == row2[col2] {
				fs1.SetCellStyle("Sheet1", fmt.Sprintf("A%v", rindex+1), fmt.Sprintf("AA%v", rindex+1), style)
			}
		}
	}
	if err := fs1.Save(); err != nil {
		return fmt.Sprintf("更新文件 %s 发送错误 %b ", fileName1, err.Error())
	}
	return "更新完成";
}

func main() {
	args := []string{}
	if runtime.GOOS == "linux" {
		args = append(args, "--class=Lorca")
	}
	ui, err := lorca.New("", "", 800, 640, args...)
	if err != nil {
		log.Fatal(err)
	}
	defer ui.Close()

	// A simple way to know when UI is ready (uses body.onload event in JS)
	ui.Bind("start", func() {
		log.Println("UI is ready")
	})

	// Create and bind Go object to the UI
	m := &msg{}
	ui.Bind("excel_diff", m.excel_diff)
	ui.Bind("msg_text", m.msg_text)

	// Load HTML.
	// You may also use `data:text/html,<base64>` approach to load initial HTML,
	// e.g: ui.Load("data:text/html," + url.PathEscape(html))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()
	go http.Serve(ln, http.FileServer(http.FS(fs)))
	ui.Load(fmt.Sprintf("http://%s/www", ln.Addr()))

	// You may use console.log to debug your JS code, it will be printed via
	// log.Println(). Also exceptions are printed in a similar manner.
	ui.Eval(`
		console.log("Hello, world!");
		console.log('Multiple values:', [1, false, {"x":5}]);
	`)

	// Wait until the interrupt signal arrives or browser window is closed
	sigc := make(chan os.Signal)
	signal.Notify(sigc, os.Interrupt)
	select {
	case <-sigc:
	case <-ui.Done():
	}

	log.Println("exiting...")
}
