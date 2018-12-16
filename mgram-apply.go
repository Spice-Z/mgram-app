package main

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	vision "cloud.google.com/go/vision/apiv1"
	"github.com/PuerkitoBio/goquery"
)

func main() {
	//　標準出力からURLを受け取る
	mgramURL := os.Args[1]
	doc, err := goquery.NewDocument(mgramURL)
	if err != nil {
		fmt.Print("url scarapping failed")
	}
	imgURL, _ := doc.Find(".image-frame > img").Attr("src")
	userName := doc.Find(".introSection > .sectionHeading").Text()

	//〇〇の診断結果→〇〇
	userName = string([]rune(userName)[0 : utf8.RuneCountInString(userName)-5])

	personalities, _ := detectTextURI(imgURL)
	sendToSheet(userName, mgramURL, "GASのURL", personalities)
}

func detectTextURI(imgURL string) (personalities []string, err error) {
	fmt.Println("using vision api")
	ctx := context.Background()
	client, err := vision.NewImageAnnotatorClient(ctx)
	if err != nil {
		return nil, err
	}
	response, err := http.Get(imgURL)
	if err != nil {
		return
	}
	defer response.Body.Close()
	srcImg, _, err := image.Decode(response.Body)
	if err != nil {
		return
	}
	srcBounds := srcImg.Bounds()
	//書き出し用イメージ
	dest := image.NewRGBA(srcBounds)
	for v := srcBounds.Min.Y; v < srcBounds.Max.Y; v++ {
		for h := srcBounds.Min.X; h < srcBounds.Max.X; h++ {
			// dest.Set(h, v, srcImg.At(h, v))
			c := color.GrayModel.Convert(srcImg.At(h, v))
			gray, _ := c.(color.Gray)
			// しきい値で二値化
			if gray.Y > 250 {
				gray.Y = 255
			} else {
				gray.Y = 0
			}
			dest.Set(h, v, gray)
			//真ん中はさようなら
			if srcBounds.Max.Y/3 < v && v < srcBounds.Max.Y*2/3 && srcBounds.Max.X/3 < h && h < srcBounds.Max.X*2/3 {
				dest.Set(h, v, color.RGBA{255, 0, 0, 0})
			}
		}
	}

	outfile, _ := os.Create("temp.png")
	defer outfile.Close()
	png.Encode(outfile, dest)

	imgPath, _ := os.Open("temp.png")
	defer imgPath.Close()

	image, err := vision.NewImageFromReader(imgPath)

	texts, err := client.DetectTexts(ctx, image, nil, 1000)
	if err != nil {
		fmt.Println("error is occured")
		log.Fatal(err)
		return nil, err
	}

	if len(texts) == 0 {
		fmt.Println("No text found.")
		return nil, nil
	}
	textCandidate := strings.Split(texts[0].Description, "\n")
	for _, annotation := range texts {
		fmt.Println(annotation.Description)
	}
	for _, value := range textCandidate {
		fmt.Println(value)
		if strings.HasPrefix(value, "#") && len(value) > 0 && !checkRegexp(`[A-Za-z]`, value) {
			personalities = append(personalities, string([]rune(value)[1:]))
		}
	}

	fmt.Println("vision api successed")

	os.Remove("temp.png")
	return
}

func sendToSheet(name string, mgramURL string, sheetURL string, personalities []string) error {
	fmt.Println("GAS using ")

	params := "?name="
	params += url.QueryEscape(name)
	params += "&mgramURL="
	params += url.QueryEscape(mgramURL)
	for key, value := range personalities {
		params += "&p"
		params += strconv.Itoa((key + 1))
		params += "="
		params += url.QueryEscape(value)
	}

	// decode
	decodedParams, _ := url.QueryUnescape(params)
	fmt.Println("▼▼▼▼GET request will send with this params ▼▼▼▼")
	fmt.Println(decodedParams)

	resp, error := http.Get(sheetURL + params)
	if error != nil {
		fmt.Println("errror occured")
		return error
	}

	defer resp.Body.Close()

	byteArray, _ := ioutil.ReadAll(resp.Body)
	fmt.Println(string(byteArray))

	fmt.Println("GAS DONE")

	return nil
}

func checkRegexp(reg, str string) bool {
	return (regexp.MustCompile(reg).Match([]byte(str)))
}
