// Package score
package score

import (
	"errors"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	fcext "github.com/FloatTech/floatbox/ctxext"
	ctrl "github.com/FloatTech/zbpctrl"
	nano "github.com/fumiama/NanoBot"

	// 数据库
	wallet "Romin/plugins/wallet"
	"Romin/src/file"
	sql "Romin/src/sqlite"
)

type score struct {
	db *sql.Sqlite
	sync.RWMutex
}

// 用户数据信息
type userdata struct {
	Uid        int64  // `Userid`
	UpdatedAt  int64  // `签到时间`
	Continuous int    // `连续签到次数`
	Level      int    // `决斗者等级`
	Picname    string // `签到图片`
}

const scoreMax = 1200

var (
	scoredata = &score{
		db: &sql.Sqlite{},
	}
	/************************************10*****20******30*****40*****50******60*****70*****80******90**************/
	/*************************2******10*****20******40*****70*****110******160******220***290*****370*******460***************/
	levelrank = [...]string{"新手", "青铜", "白银", "黄金", "白金Ⅲ", "白金Ⅱ", "白金Ⅰ", "传奇Ⅲ", "传奇Ⅱ", "传奇Ⅰ", "决斗王"}
	engine    = nano.Register("score", &ctrl.Options[*nano.Ctx]{
		DisableOnDefault:  false,
		Brief:             "签到",
		Help:              "- /签到",
		PrivateDataFolder: "score",
	})
	cachePath = engine.DataFolder() + "cache/"
)

func init() {
	go func() {
		err := os.MkdirAll(cachePath, 0755)
		if err != nil {
			panic(err)
		}
		err = os.MkdirAll(cachePath+"other/", 0755)
		if err != nil {
			panic(err)
		}
	}()
	getdb := fcext.DoOnceOnSuccess(func(ctx *nano.Ctx) bool {
		scoredata.db.DBPath = engine.DataFolder() + "score.db"
		err := scoredata.db.Open(time.Hour * 24)
		if err != nil {
			_, _ = ctx.SendPlainMessage(false, "[init ERROR]:", err)
			return false
		}
		err = scoredata.db.Create("score", &userdata{})
		if err != nil {
			_, _ = ctx.SendPlainMessage(false, "[ERROR]:", err)
			return false
		}
		return true
	})

	engine.OnMessageCommand("签到", getdb).SetBlock(true).Handle(func(ctx *nano.Ctx) {
		uid := int64(ctx.UserID())

		userinfo := scoredata.getData(uid)
		userinfo.Uid = uid

		lasttime := time.Unix(userinfo.UpdatedAt, 0)
		// 判断是否已经签到过了
		if time.Now().Format("2006/01/02") == lasttime.Format("2006/01/02") {
			if userinfo.Picname != "" {
				ctx.SendChain(nano.ReplyTo(ctx.Message.ID), nano.Text("今天已经签到过了"), nano.Image(file.BOTPATH+userinfo.Picname))
				return
			}
		}
		picFile, err := initPic()
		if err != nil {
			_, _ = ctx.SendPlainMessage(false, "[ERROR]:", err)
			return
		}
		if picFile == "" {
			_, _ = ctx.SendPlainMessage(false, "[ERROR]:图片抽取为空")
			return
		}
		add := rand.Intn(10 + userinfo.Level/2)
		subtime := time.Since(lasttime).Hours()
		if subtime > 48 {
			userinfo.Continuous = 1
		} else {
			userinfo.Continuous += 1
			add = int(math.Min(5, float64(userinfo.Continuous)))
		}
		userinfo.UpdatedAt = time.Now().Unix()
		if userinfo.Level < scoreMax {
			userinfo.Level += add
		}
		if err := scoredata.setData(userinfo); err != nil {
			ctx.SendPlainMessage(true, "签到记录失败。\n", err)
			return
		}
		level, nextLV := getLevel(userinfo.Level)
		if err := wallet.InsertWalletOf(int64(uid), add+level*5); err != nil {
			ctx.SendPlainMessage(true, "货币记录失败。\n", err)
			return
		}
		score := wallet.GetWalletOf(int64(uid))
		helloStr := getHourWord(time.Now().Hour())
		ctx.SendChain(nano.Image(file.BOTPATH+picFile), nano.At(ctx.Message.Author.ID), nano.Text(fmt.Sprintf("%s\n连续签到次数: %d\n当前等级: %s[(%d+%d)/%d]\n当前金币(+%d): %d", helloStr, userinfo.Continuous, levelrank[level], userinfo.Level, add, nextLV, add+level*5, score)))
	})
}

// 获取签到数据
func (sdb *score) getData(uid int64) (userinfo userdata) {
	sdb.Lock()
	defer sdb.Unlock()
	_ = sdb.db.Find("score", &userinfo, "where uid = "+strconv.FormatInt(uid, 10))
	return
}

// 保存签到数据
func (sdb *score) setData(userinfo userdata) error {
	sdb.Lock()
	defer sdb.Unlock()
	return sdb.db.Insert("score", &userinfo)

}

// 下载图片
func initPic() (picFile string, err error) {
	picFile, err = file.DownloadTo("https://img.moehu.org/pic.php", cachePath)
	if err != nil {
		fmt.Println("[score] 下载图片失败,将从下载其他二次元图片:", err)
		return otherPic()
	}
	return
}

// 下载图片
func otherPic() (picFile string, err error) {
	apiList := []string{"http://81.70.100.130/api/DmImg.php", "http://81.70.100.130/api/acgimg.php"}
	picFile, err = file.DownloadTo(apiList[rand.Intn(len(apiList))], cachePath)
	if err != nil {
		fmt.Println("[score] 下载图片失败,将从本地抽取:", err)
		return randFile(3)
	}
	return
}

func randFile(indexMax int) (string, error) {
	files, err := os.ReadDir(cachePath)
	if err != nil {
		return "", err
	}
	if len(files) > 0 {
		drawFile := files[rand.Intn(len(files))].Name()
		// 如果是文件夹就递归
		before, _, ok := strings.Cut(drawFile, ".")
		if !ok || before == "" {
			indexMax--
			if indexMax <= 0 {
				return "", errors.New("存在太多非图片文件,请清理~")
			}
			return randFile(indexMax)
		}
		return cachePath + drawFile, err
	}
	return "", errors.New("不存在本地签到图片")
}

func getLevel(count int) (int, int) {
	switch {
	case count < 5:
		return 0, 5
	case count > scoreMax:
		return len(levelrank) - 1, scoreMax
	default:
		for k, i := 1, 10; i <= scoreMax; i += (k * 10) * scoreMax / 460 {
			if count < i {
				return k, i
			}
			k++
		}
	}
	return -1, -1
}

func getHourWord(h int) string {
	switch {
	case 6 <= h && h < 12:
		return "早上好"
	case 12 <= h && h < 14:
		return "中午好"
	case 14 <= h && h < 19:
		return "下午好"
	case 19 <= h && h < 24:
		return "晚上好"
	case 0 <= h && h < 6:
		return "凌晨好"
	default:
		return ""
	}
}
