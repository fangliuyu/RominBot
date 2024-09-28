// Package ygo 一些关于ygo的插件
package ygo

import (
	"io"
	"math/rand"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"archive/zip"
	"encoding/json"

	"github.com/FloatTech/floatbox/binary"
	ctrl "github.com/FloatTech/zbpctrl"
	nano "github.com/fumiama/NanoBot"

	"Romin/src/file"
	"Romin/src/web"
)

const (
	serviceErr = "[ygocdb]error:"
	api        = "https://ygocdb.com/api/v0/?search="
	picherf    = "https://cdn.233.momobako.com/ygopro/pics/"
)

type searchResult struct {
	Result []cardInfo `json:"result"`
}

type cardInfo struct {
	Cid    int    `json:"cid"`
	ID     int    `json:"id"`
	CnName string `json:"cn_name"`
	ScName string `json:"sc_name"`
	MdName string `json:"md_name"`
	NwbbsN string `json:"nwbbs_n"`
	CnocgN string `json:"cnocg_n"`
	JpRuby string `json:"jp_ruby"`
	JpName string `json:"jp_name"`
	EnName string `json:"en_name"`
	Text   struct {
		Types string `json:"types"`
		Pdesc string `json:"pdesc"`
		Desc  string `json:"desc"`
	} `json:"text"`
	Data struct {
		Ot        int `json:"ot"`
		Setcode   int `json:"setcode"`
		Type      int `json:"type"`
		Atk       int `json:"atk"`
		Def       int `json:"def"`
		Level     int `json:"level"`
		Race      int `json:"race"`
		Attribute int `json:"attribute"`
	} `json:"data"`
}

var (
	en = nano.Register("ygocdb", &ctrl.Options[*nano.Ctx]{
		DisableOnDefault: false,
		Brief:            "百鸽查卡", // 本插件基于游戏王百鸽API"https://www.ygo-sem.cn/"
		Help: "- /查卡 [xxx]\n" +
			"- 随机一卡",
		PrivateDataFolder: "ygocdb",
	})
	zipfile       = en.DataFolder() + "ygocdb.com.cards.zip"
	verFile       = en.DataFolder() + "version.txt"
	cachePath     = en.DataFolder() + "pics/"
	lastVersion   = "123"
	lastTime      = 0
	lock          = sync.Mutex{}
	localJSONData = make(map[string]cardInfo)
	cradList      []string
)

func init() {
	go func() {
		if file.IsNotExist(zipfile) {
			_, err := file.DownloadTo("https://ygocdb.com/api/v0/cards.zip", en.DataFolder())
			if err != nil {
				panic(err)
			}
		}
		err := parsezip(zipfile)
		if err != nil {
			panic(err)
		}
		if file.IsNotExist(verFile) {
			data, err := web.GetData("https://ygocdb.com/api/v0/cards.zip.md5?callback=gu")
			if err != nil {
				panic(err)
			}
			lastTime = time.Now().Day()
			lastVersion = binary.BytesToString(data)
			fileData := binary.StringToBytes(strconv.Itoa(lastTime) + "\n" + lastVersion)
			err = os.WriteFile(verFile, fileData, 0644)
			if err != nil {
				panic(err)
			}
		} else {
			data, err := os.ReadFile(verFile)
			if err != nil {
				panic(err)
			}
			info := strings.Split(binary.BytesToString(data), "\n")
			time, err := strconv.Atoi(info[0])
			if err != nil {
				panic(err)
			}
			lastTime = time
			lastVersion = info[1]
		}
		err = os.MkdirAll(cachePath, 0755)
		if err != nil {
			panic(err)
		}
	}()
	en.OnMessageRegex(`^查卡\s?(.*)`).SetBlock(true).Handle(func(ctx *nano.Ctx) {
		uid := ctx.Message.Author.ID
		ctxtext := ctx.State["regex_matched"].([]string)[1]
		if ctxtext == "" {
			ctx.SendChain(nano.Text("你是想查询「空手假象」吗？"))
			return
		}
		data, err := web.GetData(api + url.QueryEscape(ctxtext))
		if err != nil {
			ctx.SendChain(nano.Text(serviceErr, err))
			return
		}
		var result searchResult
		err = json.Unmarshal(data, &result)
		if err != nil {
			ctx.SendChain(nano.Text(serviceErr, err))
			return
		}
		maxpage := len(result.Result)
		switch {
		case maxpage == 0:
			ctx.SendChain(nano.Text("没有找到相关的卡片额"))
			return
		case maxpage == 1:
			cardtextout := cardtext(result, 0)
			ctx.SendChain(nano.Image(picherf+strconv.Itoa(result.Result[0].ID)+".jpg"), nano.Text(cardtextout))
			return
		}
		var listName []string
		var listid []int
		for _, v := range result.Result {
			listName = append(listName, strconv.Itoa(len(listName))+"."+v.CnName)
			listid = append(listid, v.ID)
		}
		var (
			currentPage = 10
			nextpage    = 0
		)
		if maxpage < 10 {
			currentPage = maxpage
		}
		ctx.SendChain(nano.Text("找到", strconv.Itoa(maxpage), "张相关卡片,当前显示以下卡名：\n",
			strings.Join(listName[:currentPage], "\n"),
			"\n————————————\n输入对应数字获取卡片信息,",
			"\n或回复“取消”、“下一页”指令"))
		recv, cancel := nano.NewFutureEvent("nano", 999, false, nano.RegexRule(`(取消)|(下一页)|\d+`), nano.CheckUser(uid)).Repeat()
		after := time.NewTimer(20 * time.Second)
		for {
			select {
			case <-after.C:
				cancel()
				ctx.Send(
					nano.ReplyWithMessage(ctx.Message.ID,
						nano.Text("等待超时,搜索结束"),
					),
				)
				return
			case e := <-recv:
				nextcmd := e.Message.String()
				switch nextcmd {
				case "取消":
					cancel()
					after.Stop()
					ctx.Send(
						nano.ReplyWithMessage(ctx.Message.ID,
							nano.Text("用户取消,搜索结束"),
						),
					)
					return
				case "下一页":
					after.Reset(20 * time.Second)
					if maxpage < 11 {
						continue
					}
					nextpage++
					if nextpage*10 >= maxpage {
						nextpage = 0
						currentPage = 10
						ctx.SendChain(nano.Text("已是最后一页，返回到第一页"))
					} else if nextpage == maxpage/10 {
						currentPage = maxpage % 10
					}
					ctx.SendChain(nano.Text("找到", strconv.Itoa(maxpage), "张相关卡片,当前显示以下卡名：\n",
						strings.Join(listName[nextpage*10:nextpage*10+currentPage], "\n"),
						"\n————————————————\n输入对应数字获取卡片信息,",
						"\n或回复“取消”、“下一页”指令"))
				default:
					cardint, err := strconv.Atoi(nextcmd)
					switch {
					case err != nil:
						after.Reset(20 * time.Second)
						ctx.SendChain(nano.Text("请输入正确的序号"))
					default:
						if cardint < nextpage*10+currentPage {
							cancel()
							after.Stop()
							cardtextout := cardtext(result, cardint)
							ctx.SendChain(nano.Image(picherf+strconv.Itoa(listid[cardint])+".jpg"), nano.Text(cardtextout))
							return
						}
						after.Reset(20 * time.Second)
						ctx.SendChain(nano.Text("请输入正确的序号"))
					}
				}
			}
		}
	})
	en.OnMessageFullMatch("ycb更新", nano.SuperUserPermission).SetBlock(true).Handle(func(ctx *nano.Ctx) {
		m, err := downloadJson()
		if err != nil {
			ctx.SendChain(nano.Text(m, err))
			return
		}
		ctx.SendChain(nano.Text(m))
	})
	en.OnMessageFullMatch("随机一卡", func(ctx *nano.Ctx) bool {
		lock.Lock()
		defer lock.Unlock()
		if time.Now().Day() == lastTime {
			return true
		}
		downloadJson()
		return true
	}).SetBlock(true).Handle(func(ctx *nano.Ctx) {
		data := drawCard()
		list := []cardInfo{data}
		result := searchResult{Result: list}
		cardtextout := cardtext(result, 0)
		ctx.SendChain(nano.Image(picherf+strconv.Itoa(result.Result[0].ID)+".jpg"), nano.Text(cardtextout))

	})
}

func downloadJson() (info string, err error) {
	data, err := web.GetData("https://ygocdb.com/api/v0/cards.zip.md5?callback=gu")
	if err != nil {
		return "[EEORR]: ", err
	}
	version := binary.BytesToString(data)
	if version != lastVersion {
		_, err = file.DownloadTo("https://ygocdb.com/api/v0/cards.zip", en.DataFolder())
		if err != nil {
			return "[EEORR]: ", err
		}
		err = parsezip(zipfile)
		if err != nil {
			return "[EEORR]: ", err
		}
		lastTime = time.Now().Day()
		lastVersion = version
		fileData := binary.StringToBytes(strconv.Itoa(lastTime) + "\n" + lastVersion)
		err = os.WriteFile(verFile, fileData, 0644)
		if err != nil {
			return "[EEORR]: ", err
		}
		return "更新完成", nil
	}
	return "无需更新", nil
}

func parsezip(zipFile string) error {
	zipReader, err := zip.OpenReader(zipFile) // will not close
	if err != nil {
		return err
	}
	defer zipReader.Close()
	file, err := zipReader.File[0].Open()
	if err != nil {
		return err
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, &localJSONData)
	if err != nil {
		return err
	}
	cradList = []string{}
	for key := range localJSONData {
		cradList = append(cradList, key)
	}
	return nil
}

func cardtext(list searchResult, cardid int) string {
	var cardtext []string
	name := "C N卡名: " + list.Result[cardid].CnName
	cardtext = append(cardtext, name)
	if list.Result[cardid].NwbbsN != "" {
		name = "N W卡名: " + list.Result[cardid].NwbbsN
		cardtext = append(cardtext, name)
	}
	if list.Result[cardid].CnocgN != "" {
		name = "简中卡名: " + list.Result[cardid].CnocgN
		cardtext = append(cardtext, name)
	}
	if list.Result[cardid].NwbbsN != "" {
		name = "M D卡名: " + list.Result[cardid].MdName
		cardtext = append(cardtext, name)
	}
	if list.Result[cardid].JpName != "" {
		name = "日本卡名:"
		if list.Result[cardid].JpRuby != "" && list.Result[cardid].JpName != list.Result[cardid].JpRuby {
			name += "\n    " + list.Result[cardid].JpRuby
		}
		name += "\n    " + list.Result[cardid].JpName
		cardtext = append(cardtext, name)
	}
	if list.Result[cardid].EnName != "" {
		name = "英文卡名:\n    " + list.Result[cardid].EnName
		cardtext = append(cardtext, name)
	}
	if list.Result[cardid].ScName != "" {
		name = "其他译名: " + list.Result[cardid].ScName
		cardtext = append(cardtext, name)
	}
	cardtext = append(cardtext, "卡片密码："+strconv.Itoa(list.Result[cardid].ID))
	cardtext = append(cardtext, list.Result[cardid].Text.Types)
	if list.Result[cardid].Text.Pdesc != "" {
		cardtext = append(cardtext, "[灵摆效果]\n"+list.Result[cardid].Text.Pdesc)
		if strings.Contains(list.Result[cardid].Text.Types, "效果") {
			cardtext = append(cardtext, "[怪兽效果]")
		} else {
			cardtext = append(cardtext, "[怪兽描述]")
		}
	}
	cardtext = append(cardtext, list.Result[cardid].Text.Desc)
	return strings.Join(cardtext, "\n")
}

func drawCard(index ...int) cardInfo {
	data := cardInfo{}
	pageMax := len(cradList)
	if pageMax > 0 {
		data = localJSONData[cradList[rand.Intn(pageMax)]]
	}
	i := 0
	if len(index) > 0 {
		i = index[0]
	}
	if i > 10 {
		return data
	}
	if data.ID == 0 {
		i++
		data = drawCard(i)
	}
	return data
}
