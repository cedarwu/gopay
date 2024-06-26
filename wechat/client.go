package wechat

import (
	"context"
	"crypto/tls"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/cedarwu/gopay"
	"github.com/cedarwu/gopay/pkg/util"
	"github.com/cedarwu/gopay/pkg/xhttp"
	"github.com/cedarwu/gopay/pkg/xlog"
)

type Client struct {
	AppId       string
	MchId       string
	ApiKey      string
	BaseURL     string
	IsProd      bool
	HttpClient  *http.Client
	DebugSwitch gopay.DebugSwitch
	certificate *tls.Certificate
	mu          sync.RWMutex
}

// 初始化微信客户端 V2
//
//	appId：应用ID
//	mchId：商户ID
//	ApiKey：API秘钥值
//	IsProd：是否是正式环境
func NewClient(appId, mchId, apiKey string, isProd bool) (client *Client) {
	return &Client{
		AppId:       appId,
		MchId:       mchId,
		ApiKey:      apiKey,
		IsProd:      isProd,
		DebugSwitch: gopay.DebugOff,
	}
}

func NewClientFromHttpClient(appId, mchId, apiKey string, isProd bool, httpClient *http.Client) (client *Client) {
	return &Client{
		AppId:       appId,
		MchId:       mchId,
		ApiKey:      apiKey,
		IsProd:      isProd,
		HttpClient:  httpClient,
		DebugSwitch: gopay.DebugOff,
	}
}

// 向微信发送Post请求，对于本库未提供的微信API，可自行实现，通过此方法发送请求
//
//	bm：请求参数的BodyMap
//	path：接口地址去掉baseURL的path，例如：url为https://api.mch.weixin.qq.com/pay/micropay，只需传 pay/micropay
//	tlsConfig：tls配置，如无需证书请求，传nil
func (w *Client) PostWeChatAPISelf(ctx context.Context, bm gopay.BodyMap, path string, tlsConfig *tls.Config) (bs []byte, url string, statusCode int, header http.Header, err error) {
	return w.doProdPost(ctx, bm, path, tlsConfig)
}

// 授权码查询openid（正式）
//
//	文档地址：https://pay.weixin.qq.com/wiki/doc/api/wxpay_v2/open/chapter4_8.shtml
func (w *Client) AuthCodeToOpenId(ctx context.Context, bm gopay.BodyMap) (wxRsp *AuthCodeToOpenIdResponse, bs []byte, url string, statusCode int, header http.Header, err error) {
	err = bm.CheckEmptyError("nonce_str", "auth_code")
	if err != nil {
		return nil, nil, "", 0, nil, err
	}

	bs, url, statusCode, header, err = w.doProdPost(ctx, bm, authCodeToOpenid, nil)
	if err != nil {
		return nil, bs, url, statusCode, header, err
	}
	wxRsp = new(AuthCodeToOpenIdResponse)
	if err = xml.Unmarshal(bs, wxRsp); err != nil {
		return nil, bs, url, statusCode, header, fmt.Errorf("xml.Unmarshal(%s): %w", string(bs), err)
	}
	return wxRsp, bs, url, statusCode, header, nil
}

// 下载对账单
//
//	文档地址：https://pay.weixin.qq.com/wiki/doc/api/wxpay_v2/open/chapter3_6.shtml
func (w *Client) DownloadBill(bm gopay.BodyMap) (wxRsp string, header http.Header, err error) {
	err = bm.CheckEmptyError("nonce_str", "bill_date", "bill_type")
	if err != nil {
		return util.NULL, nil, err
	}
	billType := bm.GetString("bill_type")
	if billType != "ALL" && billType != "SUCCESS" && billType != "REFUND" && billType != "RECHARGE_REFUND" {
		return util.NULL, nil, errors.New("bill_type error, please reference: https://pay.weixin.qq.com/wiki/doc/api/jsapi.php?chapter=9_6")
	}
	var bs []byte
	if w.IsProd {
		bs, _, _, header, err = w.doProdPost(context.Background(), bm, downloadBill, nil)
	} else {
		bs, _, _, header, err = w.doSanBoxPost(context.Background(), bm, sandboxDownloadBill)
	}
	if err != nil {
		return util.NULL, header, err
	}
	return string(bs), header, nil
}

// 下载资金账单（正式）
//
//	注意：请在初始化client时，调用 client 添加证书的相关方法添加证书
//	不支持沙箱环境，因为沙箱环境默认需要用MD5签名，但是此接口仅支持HMAC-SHA256签名
//	文档地址：https://pay.weixin.qq.com/wiki/doc/api/wxpay_v2/open/chapter3_7.shtml
func (w *Client) DownloadFundFlow(bm gopay.BodyMap) (wxRsp string, header http.Header, err error) {
	err = bm.CheckEmptyError("nonce_str", "bill_date", "account_type")
	if err != nil {
		return util.NULL, nil, err
	}
	accountType := bm.GetString("account_type")
	if accountType != "Basic" && accountType != "Operation" && accountType != "Fees" {
		return util.NULL, nil, errors.New("account_type error, please reference: https://pay.weixin.qq.com/wiki/doc/api/jsapi.php?chapter=9_18&index=7")
	}
	bm.Set("sign_type", SignType_HMAC_SHA256)
	tlsConfig, err := w.addCertConfig(nil, nil, nil)
	if err != nil {
		return util.NULL, nil, err
	}
	bs, _, _, header, err := w.doProdPost(context.Background(), bm, downloadFundFlow, tlsConfig)
	if err != nil {
		return util.NULL, header, err
	}
	wxRsp = string(bs)
	return
}

// 交易保障
//
//	文档地址：（JSAPI）https://pay.weixin.qq.com/wiki/doc/api/wxpay_v2/open/chapter3_9.shtml
//	文档地址：（付款码）https://pay.weixin.qq.com/wiki/doc/api/wxpay_v2/open/chapter4_9.shtml
//	文档地址：（Native）https://pay.weixin.qq.com/wiki/doc/api/wxpay_v2/open/chapter6_9.shtml
//	文档地址：（APP）https://pay.weixin.qq.com/wiki/doc/api/wxpay_v2/open/chapter7_9.shtml
//	文档地址：（H5）https://pay.weixin.qq.com/wiki/doc/api/wxpay_v2/open/chapter8_9.shtml
//	文档地址：（微信小程序）https://pay.weixin.qq.com/wiki/doc/api/wxpay_v2/open/chapter5_9.shtml
func (w *Client) Report(bm gopay.BodyMap) (wxRsp *ReportResponse, header http.Header, err error) {
	err = bm.CheckEmptyError("nonce_str", "interface_url", "execute_time", "return_code", "return_msg", "result_code", "user_ip")
	if err != nil {
		return nil, nil, err
	}
	var bs []byte
	if w.IsProd {
		bs, _, _, header, err = w.doProdPost(context.Background(), bm, report, nil)
	} else {
		bs, _, _, header, err = w.doSanBoxPost(context.Background(), bm, sandboxReport)
	}
	if err != nil {
		return nil, nil, err
	}
	wxRsp = new(ReportResponse)
	if err = xml.Unmarshal(bs, wxRsp); err != nil {
		return nil, header, fmt.Errorf("xml.Unmarshal(%s)：%w", string(bs), err)
	}
	return wxRsp, header, nil
}

// 拉取订单评价数据（正式）
//
//	注意：请在初始化client时，调用 client 添加证书的相关方法添加证书
//	不支持沙箱环境，因为沙箱环境默认需要用MD5签名，但是此接口仅支持HMAC-SHA256签名
//	文档地址：https://pay.weixin.qq.com/wiki/doc/api/wxpay_v2/open/chapter3_11.shtml
func (w *Client) BatchQueryComment(bm gopay.BodyMap) (wxRsp string, header http.Header, err error) {
	err = bm.CheckEmptyError("nonce_str", "begin_time", "end_time", "offset")
	if err != nil {
		return util.NULL, nil, err
	}
	bm.Set("sign_type", SignType_HMAC_SHA256)
	tlsConfig, err := w.addCertConfig(nil, nil, nil)
	if err != nil {
		return util.NULL, nil, err
	}
	bs, _, _, header, err := w.doProdPost(context.Background(), bm, batchQueryComment, tlsConfig)
	if err != nil {
		return util.NULL, nil, err
	}
	return string(bs), header, nil
}

// doSanBoxPost sanbox环境post请求
func (w *Client) doSanBoxPost(ctx context.Context, bm gopay.BodyMap, path string) (bs []byte, url string, statusCode int, header http.Header, err error) {
	url = baseUrlCh + path
	bm.Set("appid", w.AppId)
	bm.Set("mch_id", w.MchId)

	if bm.GetString("sign") == util.NULL {
		bm.Set("sign_type", SignType_MD5)
		sign, err := GetSandBoxSign(w.MchId, w.ApiKey, bm)
		if err != nil {
			return nil, url, 0, nil, err
		}
		bm.Set("sign", sign)
	}

	if w.BaseURL != util.NULL {
		url = w.BaseURL + path
	}
	req := GenerateXml(bm)
	if w.DebugSwitch == gopay.DebugOn {
		xlog.Debugf("Wechat_Request: %s", req)
	}
	res, bs, errs := xhttp.NewClientFromHttpClient(ctx, w.HttpClient).Type(xhttp.TypeXML).Post(url).SendString(req).EndBytes()
	if len(errs) > 0 {
		return nil, url, 0, nil, errs[0]
	}
	if w.DebugSwitch == gopay.DebugOn {
		xlog.Debugf("Wechat_Response: %s%d %s%s", xlog.Red, res.StatusCode, xlog.Reset, string(bs))
	}
	if res.StatusCode != 200 {
		return nil, url, res.StatusCode, res.Header, fmt.Errorf("HTTP Request Error, StatusCode = %d", res.StatusCode)
	}
	if strings.Contains(string(bs), "HTML") || strings.Contains(string(bs), "html") {
		return nil, url, res.StatusCode, res.Header, errors.New(string(bs))
	}
	return bs, url, res.StatusCode, res.Header, nil
}

// Post请求、正式
func (w *Client) doProdPost(ctx context.Context, bm gopay.BodyMap, path string, tlsConfig *tls.Config) (bs []byte, url string, statusCode int, header http.Header, err error) {
	if strings.HasPrefix(path, "http") {
		url = path
	} else {
		url = baseUrlCh + path
	}
	if bm.GetString("appid") == util.NULL && bm.GetString("combine_appid") == util.NULL {
		bm.Set("appid", w.AppId)
	}
	if bm.GetString("mch_id") == util.NULL && bm.GetString("combine_mch_id") == util.NULL {
		bm.Set("mch_id", w.MchId)
	}
	if bm.GetString("sign") == util.NULL {
		sign := GetReleaseSign(w.ApiKey, bm.GetString("sign_type"), bm)
		bm.Set("sign", sign)
	}

	httpClient := xhttp.NewClientFromHttpClient(ctx, w.HttpClient)
	if w.IsProd && tlsConfig != nil {
		httpClient.SetTLSConfig(tlsConfig)
	}
	if w.BaseURL != util.NULL {
		url = w.BaseURL + path
	}
	req := GenerateXml(bm)
	if w.DebugSwitch == gopay.DebugOn {
		xlog.Debugf("Wechat_Request: %s", req)
	}
	res, bs, errs := httpClient.Type(xhttp.TypeXML).Post(url).SendString(req).EndBytes()
	if len(errs) > 0 {
		return nil, url, 0, nil, errs[0]
	}
	if w.DebugSwitch == gopay.DebugOn {
		xlog.Debugf("Wechat_Response: %s%d %s%s", xlog.Red, res.StatusCode, xlog.Reset, string(bs))
	}
	if res.StatusCode != 200 {
		return nil, url, res.StatusCode, res.Header, fmt.Errorf("HTTP Request Error, StatusCode = %d", res.StatusCode)
	}
	if strings.Contains(string(bs), "HTML") || strings.Contains(string(bs), "html") {
		return nil, url, res.StatusCode, res.Header, errors.New(string(bs))
	}
	return bs, url, res.StatusCode, res.Header, nil
}

func (w *Client) doProdPostPure(ctx context.Context, bm gopay.BodyMap, path string, tlsConfig *tls.Config) (bs []byte, header http.Header, err error) {
	var url = baseUrlCh + path
	httpClient := xhttp.NewClientFromHttpClient(ctx, w.HttpClient)
	if w.IsProd && tlsConfig != nil {
		httpClient.SetTLSConfig(tlsConfig)
	}
	if w.BaseURL != util.NULL {
		url = w.BaseURL + path
	}
	req := GenerateXml(bm)
	if w.DebugSwitch == gopay.DebugOn {
		xlog.Debugf("Wechat_Request: %s", req)
	}
	res, bs, errs := httpClient.Type(xhttp.TypeXML).Post(url).SendString(req).EndBytes()
	if len(errs) > 0 {
		return nil, nil, errs[0]
	}
	if w.DebugSwitch == gopay.DebugOn {
		xlog.Debugf("Wechat_Response: %s%d %s%s", xlog.Red, res.StatusCode, xlog.Reset, string(bs))
	}
	if res.StatusCode != 200 {
		return nil, res.Header, fmt.Errorf("HTTP Request Error, StatusCode = %d", res.StatusCode)
	}
	if strings.Contains(string(bs), "HTML") || strings.Contains(string(bs), "html") {
		return nil, res.Header, errors.New(string(bs))
	}
	return bs, res.Header, nil
}

// Get请求、正式
func (w *Client) doProdGet(ctx context.Context, bm gopay.BodyMap, path, signType string) (bs []byte, header http.Header, err error) {
	var url = baseUrlCh + path
	if bm.GetString("appid") == util.NULL {
		bm.Set("appid", w.AppId)
	}
	if bm.GetString("mch_id") == util.NULL {
		bm.Set("mch_id", w.MchId)
	}
	bm.Remove("sign")
	sign := GetReleaseSign(w.ApiKey, signType, bm)
	bm.Set("sign", sign)
	if w.BaseURL != util.NULL {
		url = w.BaseURL + path
	}

	if w.DebugSwitch == gopay.DebugOn {
		xlog.Debugf("Wechat_Request: %s", bm.JsonBody())
	}
	param := bm.EncodeURLParams()
	url = url + "?" + param
	res, bs, errs := xhttp.NewClientFromHttpClient(ctx, w.HttpClient).Get(url).EndBytes()
	if len(errs) > 0 {
		return nil, nil, errs[0]
	}
	if w.DebugSwitch == gopay.DebugOn {
		xlog.Debugf("Wechat_Response: %s%d %s%s", xlog.Red, res.StatusCode, xlog.Reset, string(bs))
	}
	if res.StatusCode != 200 {
		return nil, res.Header, fmt.Errorf("HTTP Request Error, StatusCode = %d", res.StatusCode)
	}
	if strings.Contains(string(bs), "HTML") || strings.Contains(string(bs), "html") {
		return nil, res.Header, errors.New(string(bs))
	}
	return bs, res.Header, nil
}
