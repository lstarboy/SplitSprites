// SplitSprites project main.go
package main

import (
	//"./uuid"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"
)

type KeyVal struct {
	XMLName xml.Name
	Xml     string `xml:",innerxml"`
}

type KeyValList struct {
	//XMLName xml.Name `xml:"dict"`
	Data []KeyVal `xml:",any"`
}

type KeyDictList struct {
	Keys  []KeyVal     `xml:"key"`
	Dicts []KeyValList `xml:"dict"`
}

type PlistInfo struct {
	//XMLName xml.Name `xml:"plist"`
	Keys  []KeyVal `xml:"dict>key"`
	Dicts []KeyVal `xml:"dict>dict"`
}

// for xml texture
type TexelInfo struct {
	Name    string //在plist中的名字
	Offset  [2]int // 对应plist中的offset值
	SrcSize [2]int //原尺寸
	Rotated bool

	//接下来4个变量对应plist中frame中整数值
	X      int //在合成图中的位置
	Y      int //在合成图中的位置
	Width  int //在合成图中的尺寸，与原尺寸相似语义， 若Rotated为true，则为在合成图中所占高度，否则才为在合成图中所占宽度
	Height int //在合成图中的尺寸，与原尺寸相似语义， 若Rotated为true，则为在合成图中所占宽度，否则才为在合成图中所占高度

	DstPos [2]int //根据Width， Height和Offset计算出得偏移量

	ShortName string //短文件名，即不带路径的文件名
	LongName  string //长文件名，即合成图名称/Name
}

type ActFrame struct {
	File   string `json:"file"`
	Offset [2]int `json:"offset"`
	Size   [2]int `json:"size"`
}

type ActInfo struct {
	Name   string      `json:"name"`
	Frames []*ActFrame `json:"frames"`
}

type TextureInfo struct {
	Name        string       `json:"name"`
	File        string       `json:"-"`
	UUID        string       `json:"-"`
	STime       string       `json:"-"`
	Width       int          `json:"-"`
	Height      int          `json:"-"`
	TexelWidth  int          `json:"width"`  //适用于所有Texel原图同样大小，此处引用第一个Texel的尺寸
	TexelHeight int          `json:"height"` //适用于所有Texel原图同样大小，此处引用第一个Texel的尺寸
	Texels      []*TexelInfo `json:"-"`
	Acts        []*ActInfo   `json:"acts"`
}

func checkError(err error) {
	if err != nil {
		fmt.Println("Fatal error ", err.Error())
		//io.WriteString(os.Stdout, "Fatal error "+err.Error())
		for i := 1; ; i++ {
			_, file, line, ok := runtime.Caller(i)
			if !ok {
				break
			}

			fmt.Printf("file=%s, line=%d\n", file, line)
		}
		os.Exit(1)
	}
}

func log(format string, args ...interface{}) {
	//io.WriteString(os.Stdout, msg)
	fmt.Printf(format, args...)
}

func decodeDict(innerXml string, pDict interface{}) error {
	return xml.Unmarshal(([]byte)("<dict>"+innerXml+"</dict>"), pDict)
}

func getImage(imgFile string) (img image.Image, err error) {
	reader, err := os.Open(imgFile)
	if err != nil {
		return
	}
	defer reader.Close()
	img, _, err = image.Decode(reader)
	return
}

func getFileBaseName(filename string) string {
	fileBaseName := filepath.Base(filename)
	idx := bytes.LastIndexAny([]byte(fileBaseName), ".")
	if idx > 0 {
		fileBaseName = string(fileBaseName[:idx])
	}

	return fileBaseName
}

func getActNameFromFrameName(frameName string) (actName string) {
	actName = ""
	idx := strings.Index(frameName, "/")
	if idx > 0 {
		actName = string([]byte(frameName)[:idx])
	}
	return
}

func decodeTextureInfos(plistFile string) (textureInfo TextureInfo, err error) {
	//err = nil
	textureInfo.Name = getFileBaseName(plistFile)
	content, err := ioutil.ReadFile(plistFile)
	if err != nil {
		return
	}
	var info PlistInfo
	err = xml.Unmarshal(content, &info)
	if err != nil {
		return
	}

	textureInfo.Texels = make([]*TexelInfo, 0, 16)
	textureInfo.Acts = make([]*ActInfo, 0, 4)
	for index, key := range info.Keys {
		switch key.Xml {
		case "frames":
			//log("frames=" + info.Dicts[index].Xml)
			var frames KeyDictList
			err = decodeDict(info.Dicts[index].Xml, &frames)
			if err != nil {
				return
			}
			for index2, key := range frames.Keys {
				//log("index2=" + strconv.Itoa(index2) + ";key=" + key.XMLName.Local + ";val=" + key.Xml + "\n")
				actName := getActNameFromFrameName(key.Xml)
				//log("actName=[%s]\n", actName)
				var actInfo *ActInfo = nil
				for _, info := range textureInfo.Acts {
					if info.Name == actName {
						actInfo = info
						break
					}
				}
				if actInfo == nil {
					actInfo = &ActInfo{Name: actName, Frames: make([]*ActFrame, 0, 4)}
					textureInfo.Acts = append(textureInfo.Acts, actInfo)
				}

				texelInfo := &TexelInfo{Name: key.Xml, ShortName: filepath.Base(key.Xml), LongName: textureInfo.Name + "/" + key.Xml}
				actFrame := &ActFrame{File: key.Xml}
				dicts := frames.Dicts[index2].Data
				for index3, val := range dicts {
					if val.XMLName.Local == "key" {
						switch val.Xml {
						case "frame":
							fmt.Sscanf(dicts[index3+1].Xml, "{{%d,%d},{%d,%d}}", &texelInfo.X, &texelInfo.Y, &texelInfo.Width, &texelInfo.Height)
						case "offset":
							fmt.Sscanf(dicts[index3+1].Xml, "{%d,%d}", &texelInfo.Offset[0], &texelInfo.Offset[1])
						case "sourceSize":
							fmt.Sscanf(dicts[index3+1].Xml, "{%d,%d}", &texelInfo.SrcSize[0], &texelInfo.SrcSize[1])
						case "rotated":
							texelInfo.Rotated = (dicts[index3+1].XMLName.Local == "true")
						}
					}
				}
				texelInfo.DstPos[0] = texelInfo.Offset[0] + (texelInfo.SrcSize[0]-texelInfo.Width)/2
				texelInfo.DstPos[1] = -texelInfo.Offset[1] + (texelInfo.SrcSize[1]-texelInfo.Height)/2
				actFrame.Offset[0] = texelInfo.DstPos[0]
				actFrame.Offset[1] = texelInfo.DstPos[1]
				actFrame.Size[0] = texelInfo.Width
				actFrame.Size[1] = texelInfo.Height
				//log("texelInfo=%v\n", *texelInfo)
				//log("actFrame=%v\n", *actFrame)

				textureInfo.Texels = append(textureInfo.Texels, texelInfo)
				actInfo.Frames = append(actInfo.Frames, actFrame)
			}
		case "metadata":
			//log("metadata=" + info.Dicts[index].Xml)
			var metadata KeyValList
			err = decodeDict(info.Dicts[index].Xml, &metadata)
			if err != nil {
				return
			}
			//log("len(metadata.Data)=" + strconv.Itoa(len(metadata.Data)) + "\n")
			for index2, val := range metadata.Data {
				//log("index2=" + strconv.Itoa(index2) + ";key=" + val.XMLName.Local + ";val=" + val.Xml + "\n")
				switch val.Xml {
				case "textureFileName":
					textureInfo.File = metadata.Data[index2+1].Xml
				case "size":
					fmt.Sscanf(metadata.Data[index2+1].Xml, "{%d,%d}", &textureInfo.Width, &textureInfo.Height)
				case "smartupdate":
					uuid := metadata.Data[index2+1].Xml
					prefix := "SmartUpdate:"
					idx := strings.Index(uuid, prefix)
					beg := idx + len(prefix)
					uuidBytes := []byte(uuid)[beg : beg+32]
					textureInfo.UUID = string(uuidBytes[0:8]) + "-" + string(uuidBytes[8:12]) + "-" + string(uuidBytes[12:16]) + "-" + string(uuidBytes[16:20]) + "-" + string(uuidBytes[20:32])
					//log("uuid=[%s]\n", textureInfo.UUID)
				}
			}
		}
	}

	//textureInfo.UUID = uuid.NewUUID().String()
	t := time.Now()
	textureInfo.STime = fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:%02d", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())
	if len(textureInfo.Texels) > 0 {
		textureInfo.TexelWidth = textureInfo.Texels[0].SrcSize[0]
		textureInfo.TexelHeight = textureInfo.Texels[0].SrcSize[1]
	}
	return
}

func mkdirAll(dir string) (err error) {
	err = os.MkdirAll(dir, 0777)
	os.Chmod(dir, 0777)
	return
}

func exportTextureInfo(outputDir string, texInfo *TextureInfo, tp *template.Template) (err error) {
	// export texture.xml
	{
		outFile := outputDir + "tex_" + texInfo.Name + ".xml"
		toFile, err := os.Create(outFile)
		if err != nil {
			return err
		}
		defer toFile.Close()

		err = tp.Execute(toFile, texInfo)
		if err != nil {
			return err
		}
	}

	//export sprite.json
	{
		outFile := outputDir + "spr_" + texInfo.Name + ".json"
		toFile, err := os.Create(outFile)
		if err != nil {
			return err
		}
		defer toFile.Close()

		content, err := json.Marshal(texInfo)
		if err != nil {
			return err
		}
		toFile.Write(content)
	}

	return err
}

func exportFrameTextures(fullImgFile, outputDir string, compact bool, textureInfo *TextureInfo) (err error) {
	img, err := getImage(fullImgFile)

	var subImg *image.RGBA
	var rc image.Rectangle
	for _, frameInfo := range textureInfo.Texels {
		//log("%v", *imgInfo)
		if frameInfo.Rotated {
			rc = image.Rect(0, 0, frameInfo.Height, frameInfo.Width)
		} else {
			rc = image.Rect(0, 0, frameInfo.Width, frameInfo.Height)
		}
		crop := image.NewRGBA(rc)
		draw.Draw(crop, crop.Bounds(), img, image.Pt(frameInfo.X, frameInfo.Y), draw.Src)
		if frameInfo.Rotated {
			rotCrop := image.NewRGBA(image.Rect(0, 0, frameInfo.Width, frameInfo.Height))
			for i := 0; i < frameInfo.Width; i++ {
				for j := 0; j < frameInfo.Height; j++ {
					rotCrop.Set(i, j, crop.At(frameInfo.Height-1-j, i))
				}
			}
			crop = rotCrop
		}

		if frameInfo.Width != frameInfo.SrcSize[0] || frameInfo.Height != frameInfo.SrcSize[1] {
			if compact {
				subImg = crop
			} else {
				subImg = image.NewRGBA(image.Rect(0, 0, frameInfo.SrcSize[0], frameInfo.SrcSize[1]))
				draw.Draw(subImg, subImg.Bounds(), image.Transparent, image.ZP, draw.Src)
				rc = image.Rect(frameInfo.DstPos[0], frameInfo.DstPos[1], frameInfo.DstPos[0]+frameInfo.Width, frameInfo.DstPos[1]+frameInfo.Height)
				draw.Draw(subImg, rc, crop, image.ZP, draw.Src)
			}
		} else {
			subImg = crop
		}

		toFile := outputDir + frameInfo.Name
		if strings.LastIndexAny(frameInfo.Name, "/") != -1 {
			toFile = filepath.FromSlash(toFile)
			toOutputDir := filepath.Dir(toFile)
			//log("toOutputDir=%s\n", toOutputDir)
			mkdirAll(toOutputDir)
		}
		toImg, err := os.Create(toFile)
		if err != nil {
			return err
		}
		defer toImg.Close()
		err = png.Encode(toImg, subImg)
		if err != nil {
			return err
		}
	}

	return
}

func split(inputFile, outputDir string, export, compact bool, tp *template.Template) error {
	textureInfo, err := decodeTextureInfos(inputFile)
	if err != nil {
		return err
	}

	if textureInfo.Name != filepath.Base(inputFile) {
		outputDir = outputDir + string(filepath.Separator) + textureInfo.Name
	}
	if outputDir[len(outputDir)-1] != filepath.Separator {
		outputDir = outputDir + string(filepath.Separator)
	}
	//log("outputDir=%s\n", outputDir)
	err = mkdirAll(outputDir)
	if err != nil {
		return err
	}

	fullImgFile := filepath.Dir(inputFile) + string(filepath.Separator) + textureInfo.File
	//log("fullImgFile=" + fullImgFile)
	err = exportFrameTextures(fullImgFile, outputDir, compact, &textureInfo)
	if err != nil {
		return err
	}

	if tp != nil {
		err = exportTextureInfo(outputDir, &textureInfo, tp)
	}

	return err
}

func splitMatrixTextures(input, outputDir string, rows, cols int) (err error) {
	img, err := getImage(input)
	//_, err = getImage(input)
	if err != nil {
		return
	}

	if outputDir[len(outputDir)-1] != filepath.Separator {
		outputDir = outputDir + string(filepath.Separator)
	}
	err = mkdirAll(outputDir)
	if err != nil {
		return
	}

	imgWidth := img.Bounds().Dx() / cols
	imgHeight := img.Bounds().Dy() / rows
	fileBaseName := getFileBaseName(input)
	rc := image.Rect(0, 0, imgWidth, imgHeight)
	log("filepath.Base(inputFile)=" + filepath.Base(input) + "\n")
	log("fileBaseName(inputFile)=" + fileBaseName + "\n")
	log("Size(inputFile)=%d, %d\n", imgWidth, imgHeight)
	for i := 0; i < rows*cols; i++ {
		pt := image.Pt((i%cols)*imgWidth, (i/cols)*imgHeight)
		crop := image.NewRGBA(rc)
		draw.Draw(crop, crop.Bounds(), img, pt, draw.Src)

		fileName := fmt.Sprintf("%s_%d.png", outputDir+fileBaseName, i)

		toImg, err := os.Create(fileName)
		if err != nil {
			break
		}
		defer toImg.Close()
		err = png.Encode(toImg, crop)
		if err != nil {
			break
		}
	}
	return
}

// 减小编译后文件大小 -ldflags "-s -w"
// 去掉debug中的`优化 -gcflags "-N -l"
func main() {
	var (
		input     string
		outputDir string
		rows      int
		cols      int
		export    bool
		compact   bool
	)
	flag.StringVar(&input, "i", "", "")
	flag.StringVar(&outputDir, "o", ".", "")
	flag.BoolVar(&export, "e", false, "")
	flag.BoolVar(&compact, "c", false, "")
	flag.IntVar(&rows, "rows", 1, "")
	flag.IntVar(&cols, "cols", 10, "")
	flag.Parse()

	/*
		export = true
		input = "test.plist"
		outputDir = "output"
	*/
	if len(input) == 0 {
		log("The usage: SplitSprites -i=[plist_file|plist_dir] [-o=output_dir] [-e=exportFlag(default=false)] [-c=compact(default=false)]\n")
		return
	}

	st, err := os.Stat(input)
	if err != nil {
		checkError(err)
	}

	var tp *template.Template
	if export {
		tp, err = template.ParseFiles("template/texture.xml")
		if err != nil {
			checkError(err)
		}
	}
	//log("12\n")
	if st.IsDir() {
		if input[len(input)-1] != filepath.Separator {
			input = input + string(filepath.Separator)
		}
		fileInfoArr, err := ioutil.ReadDir(input)
		checkError(err)
		for _, fileInfo := range fileInfoArr {
			//log("file=%s\n", fileInfo.Name())
			if ext := filepath.Ext(fileInfo.Name()); ext != ".plist" {
				continue
			}

			err = split(input+fileInfo.Name(), outputDir, export, compact, tp)
			checkError(err)
		}
	} else {
		if filepath.Ext(input) == ".plist" {
			err = split(input, outputDir, export, compact, tp)
		} else {
			err = splitMatrixTextures(input, outputDir, rows, cols)
		}
		checkError(err)
	}

	log("The splitting is successful!")
}
