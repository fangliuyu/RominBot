// Package main NanoBot-Plugin main file
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	// ----------------------- 插件列表 ------------------------ //
	_ "Romin/plugins/autowithdraw"
	_ "Romin/plugins/score"
	_ "Romin/plugins/status"
	_ "Romin/plugins/ygocdb"

	// -----------------------以下为内置依赖，勿动------------------------ //
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"github.com/FloatTech/floatbox/process"
	ctrl "github.com/FloatTech/zbpctrl"
	nano "github.com/fumiama/NanoBot"
	"github.com/wdvxdr1123/ZeroBot/extension"
	// -----------------------以上为内置依赖，勿动------------------------ //
)

func main() {
	savecfg := "./config.yaml"
	saveStatus := false
	token := flag.String("t", "", "qq api token")
	appid := flag.String("a", "", "qq appid")
	secret := flag.String("s", "", "qq secret")
	debug := flag.Bool("D", false, "enable debug-level log output")
	timeout := flag.Int("T", 60, "api timeout (s)")
	help := flag.Bool("h", false, "print this help")
	sandbox := flag.Bool("box", false, "run in sandbox api")
	onlypublic := flag.Bool("p", false, "only listen to private intent")
	shardindex := flag.Uint("shardindex", 0, "shard index")
	shardcount := flag.Uint("shardcount", 0, "shard count")
	flag.Parse()
	if *help {
		fmt.Println("Usage:")
		flag.PrintDefaults()
		os.Exit(0)
	}

	if *debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	var bot = nano.Bot{
		AppID:      *appid,
		Token:      *token,
		Secret:     *secret,
		Timeout:    time.Duration(*timeout) * time.Second,
		Intents:    uint32(nano.IntentGuildPublic),
		ShardIndex: uint8(*shardindex),
		ShardCount: uint8(*shardcount),
	}
	_, err := os.Stat(savecfg)
	if err == nil {
		f, err := os.Open(savecfg)
		if err != nil {
			logrus.Fatal(err)
		}
		dec := yaml.NewDecoder(f)
		dec.KnownFields(true)
		err = dec.Decode(&bot)
		_ = f.Close()
		if err != nil {
			fmt.Println("Usage:")
			flag.PrintDefaults()
			os.Exit(0)
		}
	} else {
		logrus.Warningln(err)
	}

	if *token != "" {
		saveStatus = true
		bot.Token = *token
	}
	if *appid != "" {
		saveStatus = true
		bot.AppID = *appid
	}
	if *secret != "" {
		saveStatus = true
		bot.Secret = *secret
	}

	// bot.Intents = uint32(bot.Intents)
	if bot.Intents == 0 {
		saveStatus = true
		bot.Intents = uint32(nano.IntentGuildPublic)
	}
	if *onlypublic {
		saveStatus = true
		bot.Intents = uint32(nano.IntentGuildPrivate)
	}
	if *timeout != 60 {
		saveStatus = true
		bot.Timeout = time.Duration(*timeout) * time.Second
	}

	sus := make([]string, 0, 16)
	for _, s := range flag.Args() {
		_, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			continue
		}
		saveStatus = true
		sus = append(sus, s)
	}
	bot.SuperUsers = append(bot.SuperUsers, sus...)

	if *sandbox {
		saveStatus = true
		nano.OpenAPI = nano.SandboxAPI
	}
	if *shardindex != 0 && *shardcount != 0 {
		saveStatus = true
		bot.ShardCount = uint8(*shardcount)
		bot.ShardIndex = uint8(*shardindex)
	}
	bot.Properties = nil
	if saveStatus {
		f, err := os.Create(savecfg)
		if err != nil {
			logrus.Fatal(err)
		}
		err = yaml.NewEncoder(f).Encode(&bot)
		_ = f.Close()
		if err != nil {
			logrus.Fatal(err)
		}
		logrus.Infoln("已将当前配置保存到", savecfg)
	}
	nano.OnMessageCommandGroup([]string{"help", "usage"}).SetBlock(true).Handle(func(ctx *nano.Ctx) {
		grp := ctx.GroupID()
		if grp == 0 {
			return
		}
		model := extension.CommandModel{}
		_ = ctx.Parse(&model)
		if model.Args == "" {
			grp := ctx.GroupID()
			msgs := make([]any, 0)
			msgs = append(msgs, "\n---服务详情---\n")
			nano.ForEachByPrio(func(i int, manager *ctrl.Control[*nano.Ctx]) bool {
				msgs = append(msgs, i+1, ": ", manager.EnableMarkIn(int64(grp)), manager.Service, "(", manager.Options.Brief, ")", "\n", manager.Options.Help, "\n\n")
				return true
			})
			_, _ = ctx.SendPlainMessage(false, msgs...)
			return
		}
		service, ok := nano.Lookup(model.Args)
		if !ok {
			_, _ = ctx.SendPlainMessage(false, "没有找到指定服务!")
			return
		}
		if service.Options.Help != "" {
			_, _ = ctx.SendPlainMessage(false, "\n", service.EnableMarkIn(int64(grp)), " ", service)
		} else {
			_, _ = ctx.SendPlainMessage(false, "该服务无帮助!")
		}
	})
	nano.OnMessageCommand("expose").SetBlock(true).
		Handle(func(ctx *nano.Ctx) {
			msg := ""
			if nano.OnlyQQ(ctx) {
				msg = "*报告*\n- 群ID: `" + strconv.FormatInt(int64(ctx.GroupID()), 10) + "`\n- 触发用户ID: `" + strconv.FormatInt(int64(ctx.UserID()), 10) + "`"
				for _, e := range strings.Split(ctx.State["args"].(string), " ") {
					e = strings.TrimSpace(e)
					if e == "" {
						continue
					}
					if strings.HasPrefix(e, "<@!") {
						uid := strings.TrimSuffix(e[3:], ">")
						msg += "\n- 用户: " + e + " ID: `" + uid + "`"
					}
				}
			} else {
				msg = "*报告*\n- 频道ID: `" + ctx.Message.ChannelID + "`"
				for _, e := range strings.Split(ctx.State["args"].(string), " ") {
					e = strings.TrimSpace(e)
					if e == "" {
						continue
					}
					if strings.HasPrefix(e, "<@!") {
						uid := strings.TrimSuffix(e[3:], ">")
						msg += "\n- 用户: " + e + " ID: `" + uid + "`"
					}
				}
			}
			_, _ = ctx.SendPlainMessage(true, msg)
		})
	_ = nano.Run(process.GlobalInitMutex.Unlock, &bot)
}
