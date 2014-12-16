package main

import (
	"fmt"
	"os"
	"strings"
	"strconv"
	"regexp"
	"io/ioutil"
	"net/smtp"
	"text/template"
	"bytes"
	"log"
	"errors"
	"flag"
	"encoding/json"
	"github.com/PuerkitoBio/goquery"
)

type ItemType struct {
	Id, Title, Link string
	Price int
}

type EmailUser struct {
	Username    string
	Password    string
	EmailServer string
	Port        int
}

type SmtpTemplateData struct {
	From    	string
	To      	string
	Subject 	string
	ContentType string
	Charset		string
	Body    	string
}

const emailTemplate = `From: {{.From}}
MIME-Version: 1.0
Content-Type: {{.ContentType}}; charset={{.Charset}}
To: {{.To}}
Subject: {{.Subject}}

{{.Body}}`

var (
	defaultRegion string = "moskva"
	fileName	  string = "avito-notifier.json"
	filePath	  string = "/tmp"
	link 		  string = "https://www.avito.ru/"

	savedData  []ItemType
	parsedData []ItemType
	emailUser EmailUser

    minPrice 		 	 = flag.Int("price", 0, "Price for start notifiers")
	region 				 = flag.String("region", defaultRegion, "Region for search")
	file 				 = flag.String("file", "", "File to store the results")
)

func init() {
	flag.StringVar(region, "r", defaultRegion, "Region for search")
	flag.StringVar(file, "f", "", "File to store the results")
	flag.IntVar(minPrice, "p", 0, "Price for start notifiers")


}

func main() {
	flag.Parse()
	var err error
	savedData, err = loadData(*file); errorCheck(err, "Error: Trying to load data")

	search := strings.Join(flag.Args(), "+")

	search = getUrl(region, search)

	parsedData, err = getParsedItems(search); errorCheck(err, "Error: Trying to get parsed items")

	var newItems []*ItemType
	newItemsCount := 0

	for i := range parsedData {
		if isNewItem(parsedData[i], savedData) && (*minPrice == 0 || parsedData[i].Price < *minPrice) {
			newItems = append(newItems, &(parsedData[i]))
			newItemsCount++
		}
//		fmt.Printf("%s\n", isNewItem(newItem, savedData))
	}
	s := "aasdasd"
	fmt.Printf("%d\n", len(s))

	if newItemsCount > 0 {
		notifier(newItems, newItemsCount)
		sendMail(newItems, newItemsCount)
	} else {
		fmt.Println("Новых предложений нет.")
	}

	err = saveData(parsedData, *file); errorCheck(err, "Error")
}

func notifier(items []*ItemType, count int) {
	maxTitle := 30
	titlePostfix := "..."
	fmt.Printf("Новые предложения (%d):\n", count)
	fmt.Printf("|%10s%3s%30s%3s%6s\n", "Цена", " | ", "Заголовок", " | ", "Ссылка")
	fmt.Printf("|%10s%3s%30s%3s%6s\n", "----------", "-|-", "------------------------------", "-|-", "-------")

	for _,item := range items {
		var title string
		if len(item.Title) > maxTitle {
			title = string([]rune(item.Title)[0:maxTitle-len(titlePostfix)]) + titlePostfix
		} else {
			title = item.Title
		}
		fmt.Printf("|%10d%3s%30s%3s%30s\n", item.Price, " | ", title, " | ", item.Link)
	}
}

func sendMail(items []*ItemType, count int) {

	emailUser := &EmailUser{"username", "password", "smtp.yandex.ru", 25}

	auth := smtp.PlainAuth("",
		emailUser.Username,
		emailUser.Password,
		emailUser.EmailServer)

	var err error
	var doc, body bytes.Buffer

	context := &SmtpTemplateData{
		"avito-notifier",
		"alexandr.mdr@gmail.com",
		fmt.Sprintf("[Avito Notifier]: Новые предложения (%d)", count),
		"text/html",
		"UTF-8",
		""}

	bodyTemplate := `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional //EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">
	<html><head>
		<meta http-equiv="Content-Type" content="text/html; charset={{.Charset}}" />
		<title>{{.Subject}}</title>
		<meta name="viewport" content="width=device-width, initial-scale=1.0" />
		<style>
			table {
				border-collapse: collapse;
				border: 1px solid black;
			}
  			table td, table th {
  				border: 1px solid black;
  				padding: 5px;
  			}
			table tr th {
				font-weight: bold;
			}
			.price {
				text-align: right;
			}
		</style>
	</head><body>
		<p>Появились новые предложения ({{.Count}})</p>

		<table border="1">
			<tr><th class="price">Цена</th><th>Заголовок</th></tr>
			{{range $item := .Items}}<tr>
				<td class="price"><a href="{{$item.Link}}" target="_blank">{{$item.Price}}</a></td>
				<td class="title"><a href="{{$item.Link}}" target="_blank">{{$item.Title}}</a></td>
			</tr>{{end}}
		</table>
	</body></html>`

	bodyData := struct {
		Subject string
		Charset string
		Count int
		Items []*ItemType
	} {
		context.Subject,
		context.Charset,
		count,
		items,
	}

	t := template.New("emailBodyTemplate")
	t, err = t.Parse(bodyTemplate)
	if err != nil {
		log.Print("error trying to parse mail body template")
	}
	err = t.Execute(&body, bodyData)
	if err != nil {
		log.Print("error trying to execute mail body template")
	}

	context.Body = body.String()

	t = template.New("emailTemplate")
	t, err = t.Parse(emailTemplate)
	if err != nil {
		log.Print("error trying to parse mail template")
	}
	err = t.Execute(&doc, context)
	if err != nil {
		log.Print("error trying to execute mail template")
	}

	err = smtp.SendMail(emailUser.EmailServer+":"+strconv.Itoa(emailUser.Port), // in our case, "smtp.google.com:587"
		auth,
		emailUser.Username,
		[]string{"alexandr.mdr@gmail.com"},
		doc.Bytes())
	if err != nil {
		log.Print("ERROR: attempting to send a mail ", err)
	}
}

func isNewItem(item ItemType, items []ItemType) bool {
	for _,oldItem := range items {
		if (item.Id == oldItem.Id && item.Price >= oldItem.Price) {
			return false
		}
	}
	return true
}

func loadData(file string) ([]ItemType, error) {
	filePath := getFilePath(file)
	exists, err := exists(filePath); errorCheck(err, "Error")
	if (!exists) {
		fmt.Printf("File %s does not exists", filePath)
		return []ItemType{}, nil
	}
	jsonData, err := ioutil.ReadFile(filePath); errorCheck(err, "Error")
	var data []ItemType
	json.Unmarshal(jsonData, &data)
	return data, nil
}

func saveData(items []ItemType, file string) error {
	data, err := json.Marshal(items); errorCheck(err, "Error")
	filePath := getFilePath(file)
	return ioutil.WriteFile(filePath, data, 0644)
}

func getFilePath(file string) string {
	if file == "" {
		file = os.Getenv("HOME")

		if file == "" {
			file = filePath + "/" + fileName
		} else {
			if exists,_ := exists(file + "/.config"); !exists {
				err := os.Mkdir(file + "/.config", 0755); errorCheck(err, "Error")
			}
			if exists,_ := exists(file + "/.config/avito-notifier"); !exists {
				err := os.Mkdir(file + "/.config/avito-notifier", 0755); errorCheck(err, "Error")
			}
			file += "/.config/avito-notifier/" + fileName
		}
	}

	return file
}

func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil { return true, nil }
	if os.IsNotExist(err) { return false, nil }
	return false, err
}

func getParsedItems(url string) ([]ItemType, error) {
	doc, err := goquery.NewDocument(url)
	errorCheck(err, "Error")

	var items []ItemType

	doc.Find(".l-content .clearfix .catalog .catalog-list .js-catalog_before-ads .item").Each(func(i int, s *goquery.Selection) {
		id, errGoquery := s.Attr("id")
		if !errGoquery {
			err = errors.New("Can not get attribute 'id'")
		}

		a := s.Find("h3.title a")
		title := strings.TrimSpace(a.Text())
		href, errHref := a.Attr("href")
		if !errHref {
			err = errors.New("Can not get attribute 'href'")
		}
		link := link + strings.TrimLeft(href, "/")

		price, errStrconv := strconv.Atoi(regexp.MustCompile(`[^\d]`).ReplaceAllString(s.Find(".about").Text(), ""))
		errorCheck(errStrconv, "Error")

		items = append(items, ItemType{id, title, link, price})
	})

	return items, err
}

func getUrl(region *string, search string) string {
	link := link + *region + "?q=" + strings.Replace(search, " ", "+", -1)

	return link
}

func errorCheck(e error, message string) {
	if e != nil {
		fmt.Println(message)
		panic(e)
	}
}
