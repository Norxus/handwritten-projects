package legacy

import (
	"bytes"
	"os"
	"strings"
	"text/template"
)

var glbEnvs map[string]string

// 初始化获取所有的环境变量
func init() {
	glbEnvs = make(map[string]string)
	// 获取当前进程所有的环境变量
	envs := os.Environ()
	for _, env := range envs {
		// 将每个环境变量按照等号分成两部分存入map中
		pair := strings.SplitN(env, "=", 2)
		if len(pair) != 2 {
			continue
		}
		glbEnvs[pair[0]] = pair[1]
	}
}

// 读取path路径下的模板，然后用全局变量填充它
func GetRenderedConfFromFile(path string) (out []byte, err error) {
	var b []byte
	b, err = os.ReadFile(path)
	if err != nil {
		return
	}
	out, err = RenderContent(b)
	return
}

// 输入一个模板，使用所有的全局变量填充它
func RenderContent(in []byte) (out []byte, err error) {
	// 定义一个模板对象frp，解析字符串作为模板内容并生成一个模板对象
	tmpl, errRet := template.New("frp").Parse(string(in))
	if errRet != nil {
		err = errRet
		return
	}

	buffer := bytes.NewBufferString("")
	v := GetValues()
	// 将占位符替换为v中的参数，并将渲染的结果写入到buffer中
	// 后续可以通过buffer对象的方法获取渲染结果
	err = tmpl.Execute(buffer, v)
	if err != nil {
		return
	}
	out = buffer.Bytes()
	return
}

type Values struct {
	Envs map[string]string
}

// 获取所有的全局变量
func GetValues() *Values {
	return &Values{
		Envs: glbEnvs,
	}
}
