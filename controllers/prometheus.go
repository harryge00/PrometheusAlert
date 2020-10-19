package controllers

import (
	"PrometheusAlert/model"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
)

type PrometheusController struct {
	beego.Controller
}


type Labels struct {
	Alertname string `json:"alertname"`
	Instance  string `json:"instance"`
	InstanceID  string `json:"instance_id,omitempty"`
	InstanceID2  string `json:"instanceid,omitempty"` // 腾讯云redis
	InstanceName  string `json:"instance_name,omitempty"`
	Host  string `json:"host,omitempty"`
	ServiceName  string `json:"service_name,omitempty"`
	TaskName  string `json:"task_name,omitempty"`
	Level     string `json:"level"` //2019年11月20日 16:03:10更改告警级别定义位置,适配prometheus alertmanager rule
}
type Annotations struct {
	Description string `json:"description"`
	Summary     string `json:"summary"`
	//Level string `json:"level"`  //2019年11月20日 16:04:04 删除Annotations level,改用label中的level
	Mobile string `json:"mobile"` //2019年2月25日 19:09:23 增加手机号支持
	Ddurl  string `json:"ddurl"`  //2019年3月12日 20:33:38 增加多个钉钉告警支持
	Wxurl  string `json:"wxurl"`  //2019年3月12日 20:33:38 增加多个钉钉告警支持
	Fsurl  string `json:"fsurl"`  //2020年4月25日 17:33:38 增加多个飞书告警支持
	Email  string `json:"email"`  //2020年7月4日 10:15:20 增加多个飞书告警支持
}
type Alerts struct {
	Status       string
	Labels       Labels      `json:"labels"`
	Annotations  Annotations `json:"annotations"`
	StartsAt     string      `json:"startsAt"`
	EndsAt       string      `json:"endsAt"`
	GeneratorUrl string      `json:"generatorURL"` //prometheus 告警返回地址
}
type Prometheus struct {
	Status      string
	Alerts      []Alerts
	Externalurl string `json:"externalURL"` //alertmanage 返回地址
}

// 按照 Alert.Level 从大到小排序
type AlerMessages []Alerts

func (a AlerMessages) Len() int { // 重写 Len() 方法
	return len(a)
}
func (a AlerMessages) Swap(i, j int) { // 重写 Swap() 方法
	a[i], a[j] = a[j], a[i]
}
func (a AlerMessages) Less(i, j int) bool { // 重写 Less() 方法， 从大到小排序
	return a[j].Labels.Level < a[i].Labels.Level
}

//for prometheus alert
//关于告警级别level共有5个级别,0-4,0 信息,1 警告,2 一般严重,3 严重,4 灾难
func (c *PrometheusController) PrometheusAlert() {
	alert := Prometheus{}
	logsign := "[" + LogsSign() + "]"
	logs.Info(logsign, string(c.Ctx.Input.RequestBody))
	json.Unmarshal(c.Ctx.Input.RequestBody, &alert)
	c.Data["json"] = SendMessageR(alert, "", "", "", "", "", logsign)
	logs.Info(logsign, c.Data["json"])
	c.ServeJSON()
}

func (c *PrometheusController) PrometheusRouter() {
	wxurl := c.GetString("wxurl")
	ddurl := c.GetString("ddurl")
	fsurl := c.GetString("fsurl")
	phone := c.GetString("phone")
	email := c.GetString("email")
	logsign := "[" + LogsSign() + "]"
	alert := Prometheus{}
	logs.Info(logsign, string(c.Ctx.Input.RequestBody))
	json.Unmarshal(c.Ctx.Input.RequestBody, &alert)
	c.Data["json"] = SendMessageR(alert, wxurl, ddurl, fsurl, phone, email, logsign)
	logs.Info(logsign, c.Data["json"])
	c.ServeJSON()
}

func SendMessageR(message Prometheus, rwxurl, rddurl, rfsurl, rphone, remail, logsign string) string {
	//增加日志标志  方便查询日志

	Messagelevel, _ := beego.AppConfig.Int("messagelevel")
	PhoneCalllevel, _ := beego.AppConfig.Int("phonecalllevel")
	PhoneCallResolved, _ := beego.AppConfig.Int("phonecallresolved")
	Silent, _ := beego.AppConfig.Int("silent")
	//var ddtext, wxtext, fstext, MobileMessage, PhoneCallMessage, EmailMessage, titleend string
	var MobileMessages []string
	//对分组消息进行排序
	AlerMessage := message.Alerts
	sort.Sort(AlerMessages(AlerMessage))
	//告警级别定义 0 信息,1 警告,2 一般严重,3 严重,4 灾难
	//遍历消息
	for _, RMessage := range AlerMessage {
		nLevel, _ := strconv.Atoi(RMessage.Labels.Level)

		MobileMessages = []string{
			RMessage.Labels.Alertname,
		}
		if RMessage.Annotations.Summary != "" {
			MobileMessages = append(MobileMessages, RMessage.Annotations.Summary)
		} else if RMessage.Annotations.Description != "" {
			MobileMessages = append(MobileMessages, RMessage.Annotations.Description)
		} else {
			MobileMessages = append(MobileMessages, RMessage.Labels.Alertname)
		}

		if RMessage.Labels.TaskName != "" {
			// DCOS Container alerts
			// service_name="/ntt/ds-kafka",task_name="kafka-0-broker"
			MobileMessages = append(MobileMessages, RMessage.Labels.Instance, fmt.Sprintf("%s: %s", RMessage.Labels.ServiceName, RMessage.Labels.TaskName))
		} else if RMessage.Labels.InstanceName != ""  {
			// Tencent cloud alert
			if RMessage.Labels.InstanceID != "" {
				MobileMessages = append(MobileMessages, RMessage.Labels.InstanceID, RMessage.Labels.InstanceName)
			} else {
				MobileMessages = append(MobileMessages, RMessage.Labels.InstanceID2, RMessage.Labels.InstanceName)
			}
		} else {
			// DCOS node alert
			MobileMessages = append(MobileMessages, RMessage.Labels.Instance, RMessage.Labels.Host)
		}
		MobileMessages = append(MobileMessages, RMessage.StartsAt)

		if RMessage.Status == "resolved" {
			model.AlertsFromCounter.WithLabelValues("prometheus", RMessage.Annotations.Description, RMessage.Labels.Level, RMessage.Labels.Instance, "resolved").Add(1)
			// 发生告警(1),告警内容:(2),实例ID:(3),实例名:(4),触发时间:(5),请及时处理
			MobileMessages[0] += "已恢复"
			//MobileMessage = "\n[" + Title + "Prometheus" + titleend + "]\n" + RMessage.Labels.Alertname + "\n" + "告警级别：" + AlertLevel[nLevel] + "\n" + "故障主机IP：" + RMessage.Labels.Instance + "\n" + RMessage.Annotations.Description
			PhoneCallMessage = "故障主机IP " + RMessage.Labels.Instance + RMessage.Annotations.Description + "已经恢复"

		} else {
			model.AlertsFromCounter.WithLabelValues("prometheus", RMessage.Annotations.Description, RMessage.Labels.Level, RMessage.Labels.Instance, "firing").Add(1)
			PhoneCallMessage = "故障主机IP " + RMessage.Labels.Instance + RMessage.Annotations.Description
		}

		//发送消息到短信
		if nLevel == Messagelevel {
			if rphone == "" && RMessage.Annotations.Mobile == "" {
				phone := GetUserPhone(1)
				PostTXmessage(MobileMessages, phone, logsign)
				//PostHWmessage(MobileMessage, phone, logsign)
				//PostALYmessage(MobileMessage, phone, logsign)
				//Post7MOORmessage(MobileMessage, phone, logsign)
			} else {
				if rphone != "" {
					PostTXmessage(MobileMessages, rphone, logsign)
					//PostHWmessage(MobileMessage, rphone, logsign)
					//PostALYmessage(MobileMessage, rphone, logsign)
					//Post7MOORmessage(MobileMessage, rphone, logsign)
				}
				if RMessage.Annotations.Mobile != "" {
					PostTXmessage(MobileMessages, RMessage.Annotations.Mobile, logsign)
					//PostHWmessage(MobileMessage, RMessage.Annotations.Mobile, logsign)
					//PostALYmessage(MobileMessage, RMessage.Annotations.Mobile, logsign)
					//Post7MOORmessage(MobileMessage, RMessage.Annotations.Mobile, logsign)
				}
			}
		}
		//发送消息到语音
		if nLevel == PhoneCalllevel {
			//判断如果是恢复信息且PhoneCallResolved
			if RMessage.Status == "resolved" && PhoneCallResolved != 1 {
				logs.Info(logsign, "告警恢复消息已经关闭")
			} else {
				if rphone == "" && RMessage.Annotations.Mobile == "" {
					phone := GetUserPhone(1)
					PostTXphonecall(PhoneCallMessage, phone, logsign)
					//PostALYphonecall(PhoneCallMessage, phone, logsign)
					//PostRLYphonecall(PhoneCallMessage, phone, logsign)
					//Post7MOORphonecall(PhoneCallMessage, phone, logsign)
				} else {
					if rphone != "" {
						PostTXphonecall(PhoneCallMessage, rphone, logsign)
						//PostALYphonecall(PhoneCallMessage, rphone, logsign)
						//PostRLYphonecall(PhoneCallMessage, rphone, logsign)
						//Post7MOORphonecall(PhoneCallMessage, rphone, logsign)
					}
					if RMessage.Annotations.Mobile != "" {
						PostTXphonecall(PhoneCallMessage, RMessage.Annotations.Mobile, logsign)
						//PostALYphonecall(PhoneCallMessage, RMessage.Annotations.Mobile, logsign)
						//PostRLYphonecall(PhoneCallMessage, RMessage.Annotations.Mobile, logsign)
						//Post7MOORphonecall(PhoneCallMessage, RMessage.Annotations.Mobile, logsign)
					}
				}
			}
		}
		//告警抑制开启就直接跳出循环
		if Silent == 1 {
			break
		}
	}
	return "告警消息发送完成."
}
