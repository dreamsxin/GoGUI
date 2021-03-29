package main

import (
	"embed"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"

	"github.com/360EntSecGroup-Skylar/excelize"
	"github.com/shirou/gopsutil/disk"
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

// GetDrives iterates through the alphabet to return a list of mounted drives
func GetDrives() []string {
	partitions, err := disk.Partitions(true)
	if err != nil {
		return nil
	}
	var list []string
	for _, partition := range partitions {
		log.Println(partition)
		list = append(list, partition.Device)
	}
	return list
}

func GetFiles(dirname string) []string {

	var list []string
	files, err := ioutil.ReadDir("./")
	if err != nil {
		return nil
	}
	for _, f := range files {
		log.Println(f)
		list = append(list, f.Name())
	}

	return list
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

	style1, _ := fs1.NewStyle(`{"border":[{"type":"left","color":"000000","style":1},{"type":"top","color":"000000","style":1},{"type":"bottom","color":"000000","style":1},{"type":"right","color":"000000","style":1}],"fill":{"type":"pattern","color":["#ffeb00"],"pattern":1},"alignment":{"horizontal":"left","ident":1,"vertical":"center","wrap_text":true}}`)
	style2, _ := fs2.NewStyle(`{"border":[{"type":"left","color":"000000","style":1},{"type":"top","color":"000000","style":1},{"type":"bottom","color":"000000","style":1},{"type":"right","color":"000000","style":1}],"fill":{"type":"pattern","color":["#ffeb00"],"pattern":1},"alignment":{"horizontal":"left","ident":1,"vertical":"center","wrap_text":true}}`)

	for rindex, row1 := range rows1 {
		if col1 >= len(row1) {
			continue
		}
		for rindex2, row2 := range rows2 {
			if col2 >= len(row2) {
				continue
			}
			if row1[col1] == row2[col2] {
				colName1, _ := excelize.ColumnNumberToName(col1 + 1)
				colName2, _ := excelize.ColumnNumberToName(col2 + 1)
				log.Println("file1 ", colName1, fmt.Sprintf("%v%v", colName1, rindex+1))
				log.Println("file2 ", colName2, fmt.Sprintf("%v%v", colName2, rindex2+1))
				fs1.SetCellStyle("Sheet1", fmt.Sprintf("%v%v", colName1, rindex+1), fmt.Sprintf("%v%v", colName1, rindex+1), style1)
				fs2.SetCellStyle("Sheet1", fmt.Sprintf("%v%v", colName2, rindex2+1), fmt.Sprintf("%v%v", colName2, rindex2+1), style2)
			}
		}
	}
	if err := fs1.Save(); err != nil {
		return fmt.Sprintf("更新文件 %s 发送错误 %b ", fileName1, err.Error())
	}
	if err := fs2.Save(); err != nil {
		return fmt.Sprintf("更新文件 %s 发送错误 %b ", fileName2, err.Error())
	}
	return "更新完成"
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
	ui.Bind("GetDrives", GetDrives)
	ui.Bind("GetFiles", GetFiles)

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
