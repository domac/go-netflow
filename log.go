package main

import (
	"errors"
	"fmt"
	"github.com/cihub/seelog"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var default_log_xml string = `<seelog type="asyncloop" minlevel="[LOGLEVEL]" maxlevel="error">
		<outputs formatid="rolllog">
			<rollingfile formatid="rolllog" type="size" filename="[FILENAME]" maxsize="20971520" maxrolls="2" />
        <filter levels="error">
			<rollingfile formatid="rolllog" type="size" filename="[ERROR_FILENAME]" maxsize="20971520" maxrolls="2" />
		</filter>
		</outputs>
		<formats>
			<format id="rolllog" format="%Date %Time [%l] [%Func] [%File.%Line] %Msg%n"/>
		</formats>
	</seelog>
`

var g_log seelog.LoggerInterface

func INIT_LOG(osType string, level string) {
	fmt.Printf("os:%s\n", osType)
	fmt.Printf("log level:%s\n", level)
	var LOG_PATH = "./netflow.log"
	var ERROR_LOG_PATH = "./netflow.log"
	fmt.Printf("log path:%s\n", LOG_PATH)
	g_log = GetDefaultLogger(LOG_PATH, ERROR_LOG_PATH, level)
	g_log.SetAdditionalStackDepth(1)
}

func LOG_INFO(v ...interface{}) {
	g_log.Info(v)
}

func LOG_TRACE(v ...interface{}) {
	g_log.Trace(v)
}

func LOG_DEBUG(v ...interface{}) {
	g_log.Debug(v)
}

func LOG_WARN(v ...interface{}) {
	g_log.Warn(v)
}

func LOG_ERROR(v ...interface{}) {
	g_log.Error(v)
}

func LOG_INFO_F(format string, v ...interface{}) {
	g_log.Infof(format, v...)
}

func LOG_TRACE_F(format string, v ...interface{}) {
	g_log.Tracef(format, v...)
}

func LOG_DEBUG_F(format string, v ...interface{}) {
	g_log.Debugf(format, v...)
}

func LOG_WARN_F(format string, v ...interface{}) {
	g_log.Warnf(format, v...)
}

func LOG_ERROR_F(format string, v ...interface{}) {
	g_log.Errorf(format, v...)
}

func parse_xml_conf(str string) string {
	path := GetMainDiectory()
	str = strings.Replace(str, "$(file_dir)", path, -1)
	path = GetExeFileBaseName()
	str = strings.Replace(str, "$(file_name)", path, -1)
	curtime := time.Now().Format("2006_01_02_15_04_05")
	return strings.Replace(str, "$(time)", curtime, -1)
}

func GetDefaultLogger(fileName, errFileName, level string) seelog.LoggerInterface {
	//
	//	优先使用processname_log.xml
	//	其次使用../../service_log.xml
	//	如果不存在配置文件，则使用内置日志格式
	//
	var str string

	default_log_xml = strings.Replace(default_log_xml, "[LOGLEVEL]", level, 1)
	default_log_xml = strings.Replace(default_log_xml, "[FILENAME]", fileName, 1)
	default_log_xml = strings.Replace(default_log_xml, "[ERROR_FILENAME]", errFileName, 1)

	str = default_log_xml
	log, _ := seelog.LoggerFromConfigAsString(parse_xml_conf(str))

	if log == nil {
		log = seelog.Default
	}

	return log
}

func getStack() []byte {
	buf := make([]byte, 1024)
	for {
		n := runtime.Stack(buf, false)
		if n < len(buf) {
			return buf[:n]
		}
		buf = make([]byte, 2*len(buf))
	}
}

func LOG_FLUSH() {
	if err := recover(); err != nil {
		LOG_ERROR(string(getStack()))
	}
	g_log.Flush()
}

// func init() {
// 	g_log = GetDefaultLogger()
// 	g_log.SetAdditionalStackDepth(1)
// }

//
// 获取进程路径
//
func GetExeFilePath() string {
	path, err := filepath.Abs(os.Args[0])
	if err != nil {
		LOG_ERROR(err.Error())
	}
	return path
}

//
// 获取进程名，带后缀
//
func GetExeFileName() string {
	path, err := filepath.Abs(os.Args[0])

	if err != nil {
		return ""
	}

	return filepath.Base(path)
}

//
// 获取进程名，不带后缀
//
func GetExeFileBaseName() string {
	name := GetExeFileName()
	return strings.TrimSuffix(name, filepath.Ext(name))
}

//
// 路径最后添加反斜杠
//
func PathAddBackslash(path string) string {
	i := len(path) - 1

	if !os.IsPathSeparator(path[i]) {
		path += string(os.PathSeparator)
	}

	return path
}

//
// 路径最后去除反斜杠
//
func PathRemoveBackslash(path string) string {
	i := len(path) - 1

	if i > 0 && os.IsPathSeparator(path[i]) {
		path = path[:i]
	}

	return path
}

func PathFileExists(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		return false
	}
	return true
}

//
// 获取进程所在目录: 末尾带反斜杠
//
func GetMainDiectory() string {
	path, err := filepath.Abs(os.Args[0])

	if err != nil {
		return ""
	}

	full_path := filepath.Dir(path)

	return PathAddBackslash(full_path)
}

//
// 获取进程所在目录的文件路径：filename表示文件名
//
func GetMainPath(filename string) string {
	return GetMainDiectory() + filename
}

//
// 创建目录
//
func CreateDirectory(path string) bool {
	return os.Mkdir(path, os.ModePerm) == nil
}

// @hwy
// 复制文件,目标文件路径不存在会创建
// srcpath string: 源文件
// destpath string: 目标文件
func CopyFile(srcpath string, destpath string) bool {
	src, err := os.Open(srcpath)
	if err != nil {
		LOG_ERROR_F("Failed to open srcfile, filePath:%s, err:%v.", srcpath, err)
		return false
	}
	defer src.Close()

	// 创建目标文件父目录
	destDir := filepath.Dir(destpath)
	if !PathFileExists(destDir) {
		os.MkdirAll(destDir, os.ModePerm)
	}
	dst, err := os.OpenFile(destpath, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, os.ModePerm)
	if err != nil {
		LOG_ERROR_F("Failed to open destfile, filePath:%s, err:%v.", destpath, err)
		return false
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	if err != nil {
		LOG_ERROR_F("Failed to copy file, srcfile:%s, destfile:%s, err:%v.", srcpath, destpath, err)
		return false
	}
	return true
}

// @hwy
// 获取文件名，包括后缀。如：/a/s/c.txt->c.txt; /a/s/c.txt/->空串; abs->abs;
//
func GetPathFileName(path string) string {
	path = strings.Replace(path, "\\", "/", -1)
	findex := strings.LastIndex(path, "/")

	return path[findex+1:]
}

// @hwy
// 删除目录中的文件名包含指定字符串的文件
// 返回值:
//   bool:删除失败返回false
func RemoveFile(path string, match string) bool {
	err := filepath.Walk(path, func(path string, f os.FileInfo, err error) error {
		if f == nil {
			return err
		}
		if f.IsDir() {
			return nil
		}
		if strings.Index(path, match) != -1 {
			return os.Remove(path)
		}
		return nil
	})

	if err != nil {
		return false
	}
	return true
}

// @hwy
// 清空文件夹中所有的内容，不删除该文件夹
func ClearDir(path string) bool {
	if !PathFileExists(path) {
		LOG_INFO_F("Folder does not exist, path:%s.", path)
		return true
	}
	lists, err := ioutil.ReadDir(path)
	if err != nil {
		LOG_ERROR_F("Failed to readDir, dir:%s.", path)
		return false
	}

	for _, fi := range lists {
		curpath := path + string(os.PathSeparator) + fi.Name()
		err := os.RemoveAll(curpath)
		if err != nil {
			LOG_INFO_F("Failed to execute removeAll, path:%s.", curpath)
			continue
		}
	}
	return true
}

// 拷贝文件夹
//
func CopyDir(srcDir string, destDir string) bool {
	srcDir, _ = filepath.Abs(srcDir)
	destDir, _ = filepath.Abs(destDir)
	err := filepath.Walk(srcDir, func(path string, f os.FileInfo, err error) error {
		if f == nil {
			return err
		}
		if f.IsDir() {

		} else {
			newDest := strings.Replace(path, srcDir, destDir, -1)
			LOG_INFO_F("CopyFile, srcFile:%s, destFile:%s.", path, destDir+"/"+f.Name())
			CopyFile(path, newDest)
		}
		return nil
	})
	if err != nil {
		LOG_ERROR_F("Failed to copyDir, err:%v.", err)
		return false
	}
	return true
}

// 移动源文件夹所有文件到目标文件夹中
func MoveDir(srcDir string, destDir string) bool {
	srcDir, _ = filepath.Abs(srcDir)
	destDir, _ = filepath.Abs(destDir)
	err := filepath.Walk(srcDir, func(path string, f os.FileInfo, err error) error {
		if f == nil {
			return err
		}
		if f.IsDir() {

		} else {
			newDest := strings.Replace(path, srcDir, destDir, -1)
			newDestDir := filepath.Dir(newDest)
			if !PathFileExists(newDestDir) {
				err := os.MkdirAll(newDestDir, os.ModePerm)
				if err != nil {
					LOG_ERROR_F("Failed to os.MkdirAll(%s), err:%v.", newDestDir, err)
					return errors.New("Failed to os.MkdirAll.")
				}
			}
			err := os.Rename(path, newDest)
			if err != nil {
				LOG_ERROR_F("Failed to move file, src:%s, dest:%s, err:%v.", path, newDest, err)
				return errors.New("Failed to move file.")
			}
			LOG_INFO_F("MoveFile, srcFile:%s, destFile:%s.", path, newDest)
		}
		return nil
	})
	if err != nil {
		LOG_ERROR_F("Failed to moveDir, err:%v.", err)
		return false
	}
	return true
}

func ReadFileAsString(path string) string {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}
