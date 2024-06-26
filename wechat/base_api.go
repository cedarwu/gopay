package wechat

import (
	"context"
	"crypto/tls"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"

	"github.com/cedarwu/gopay"
	"github.com/cedarwu/gopay/pkg/util"
)

// 统一下单
//
//	文档地址：https://pay.weixin.qq.com/wiki/doc/api/wxpay_v2/open/chapter3_1.shtml
func (w *Client) UnifiedOrder(ctx context.Context, bm gopay.BodyMap) (wxRsp *UnifiedOrderResponse, bs []byte, url string, statusCode int, header http.Header, err error) {
	// err = bm.CheckEmptyError("nonce_str", "body", "out_trade_no", "total_fee", "spbill_create_ip", "notify_url", "trade_type")
	// if err != nil {
	// 	return nil, nil, "", 0, nil, err
	// }
	if w.IsProd {
		bs, url, statusCode, header, err = w.doProdPost(ctx, bm, unifiedOrder, nil)
	} else {
		bm.Set("total_fee", 101)
		bs, url, statusCode, header, err = w.doSanBoxPost(ctx, bm, sandboxUnifiedOrder)
	}
	if err != nil {
		return nil, nil, url, statusCode, header, err
	}
	wxRsp = new(UnifiedOrderResponse)
	if err = xml.Unmarshal(bs, wxRsp); err != nil {
		return nil, nil, url, statusCode, header, fmt.Errorf("xml.Unmarshal(%s)：%w", string(bs), err)
	}
	return wxRsp, bs, url, statusCode, header, nil
}

// 提交付款码支付
//
//	文档地址：https://pay.weixin.qq.com/wiki/doc/api/wxpay_v2/open/chapter4_1.shtml
func (w *Client) Micropay(ctx context.Context, bm gopay.BodyMap) (wxRsp *MicropayResponse, bs []byte, url string, statusCode int, header http.Header, err error) {
	// err = bm.CheckEmptyError("nonce_str", "body", "out_trade_no", "total_fee", "spbill_create_ip", "auth_code")
	// if err != nil {
	// 	return nil, nil, "", 0, nil, err
	// }
	if w.IsProd {
		bs, url, statusCode, header, err = w.doProdPost(ctx, bm, microPay, nil)
	} else {
		bm.Set("total_fee", 1)
		bs, url, statusCode, header, err = w.doSanBoxPost(ctx, bm, sandboxMicroPay)
	}
	if err != nil {
		return nil, nil, url, statusCode, header, err
	}
	wxRsp = new(MicropayResponse)
	if err = xml.Unmarshal(bs, wxRsp); err != nil {
		return nil, bs, url, statusCode, header, fmt.Errorf("xml.Unmarshal(%s): %w", string(bs), err)
	}
	return wxRsp, bs, url, statusCode, header, nil
}

// 查询订单
//
//	文档地址：https://pay.weixin.qq.com/wiki/doc/api/wxpay_v2/open/chapter3_2.shtml
func (w *Client) QueryOrder(ctx context.Context, bm gopay.BodyMap) (wxRsp *QueryOrderResponse, bs []byte, url string, statusCode int, header http.Header, err error) {
	err = bm.CheckEmptyError("nonce_str")
	if err != nil {
		return nil, nil, "", 0, nil, err
	}
	if bm.GetString("out_trade_no") == util.NULL && bm.GetString("transaction_id") == util.NULL {
		return nil, nil, "", 0, nil, errors.New("out_trade_no and transaction_id are not allowed to be null at the same time")
	}
	if w.IsProd {
		bs, url, statusCode, header, err = w.doProdPost(ctx, bm, orderQuery, nil)
	} else {
		bs, url, statusCode, header, err = w.doSanBoxPost(ctx, bm, sandboxOrderQuery)
	}
	if err != nil {
		return nil, nil, url, statusCode, header, err
	}
	wxRsp = new(QueryOrderResponse)
	if err = xml.Unmarshal(bs, wxRsp); err != nil {
		return nil, nil, url, statusCode, header, fmt.Errorf("xml.UnmarshalStruct(%s)：%w", string(bs), err)
	}
	return wxRsp, bs, url, statusCode, header, nil
}

// 关闭订单
//
//	文档地址：https://pay.weixin.qq.com/wiki/doc/api/wxpay_v2/open/chapter3_3.shtml
func (w *Client) CloseOrder(ctx context.Context, bm gopay.BodyMap) (wxRsp *CloseOrderResponse, bs []byte, url string, statusCode int, header http.Header, err error) {
	err = bm.CheckEmptyError("nonce_str", "out_trade_no")
	if err != nil {
		return nil, nil, "", 0, nil, err
	}
	if w.IsProd {
		bs, url, statusCode, header, err = w.doProdPost(ctx, bm, closeOrder, nil)
	} else {
		bs, url, statusCode, header, err = w.doSanBoxPost(ctx, bm, sandboxCloseOrder)
	}
	if err != nil {
		return nil, nil, url, statusCode, header, err
	}
	wxRsp = new(CloseOrderResponse)
	if err = xml.Unmarshal(bs, wxRsp); err != nil {
		return nil, nil, url, statusCode, header, fmt.Errorf("xml.Unmarshal(%s)：%w", string(bs), err)
	}
	return wxRsp, bs, url, statusCode, header, nil
}

// 申请退款
//
//	注意：请在初始化client时，调用 client 添加证书的相关方法添加证书
//	文档地址：https://pay.weixin.qq.com/wiki/doc/api/wxpay_v2/open/chapter3_4.shtml
func (w *Client) Refund(ctx context.Context, bm gopay.BodyMap) (wxRsp *RefundResponse, bs []byte, url string, statusCode int, header http.Header, err error) {
	err = bm.CheckEmptyError("nonce_str", "out_refund_no", "total_fee", "refund_fee")
	if err != nil {
		return nil, nil, "", 0, nil, err
	}
	if bm.GetString("out_trade_no") == util.NULL && bm.GetString("transaction_id") == util.NULL {
		return nil, nil, "", 0, nil, errors.New("out_trade_no and transaction_id are not allowed to be null at the same time")
	}
	var (
		tlsConfig *tls.Config
	)
	if w.IsProd {
		if tlsConfig, err = w.addCertConfig(nil, nil, nil); err != nil {
			return nil, nil, "", 0, nil, err
		}
		bs, url, statusCode, header, err = w.doProdPost(ctx, bm, refund, tlsConfig)
	} else {
		bs, url, statusCode, header, err = w.doSanBoxPost(ctx, bm, sandboxRefund)
	}
	if err != nil {
		return nil, nil, url, statusCode, header, err
	}
	wxRsp = new(RefundResponse)
	if err = xml.Unmarshal(bs, wxRsp); err != nil {
		return nil, nil, url, statusCode, header, fmt.Errorf("xml.UnmarshalStruct(%s)：%w", string(bs), err)
	}
	return wxRsp, bs, url, statusCode, header, nil
}

// 查询退款
//
//	文档地址：https://pay.weixin.qq.com/wiki/doc/api/wxpay_v2/open/chapter3_5.shtml
func (w *Client) QueryRefund(ctx context.Context, bm gopay.BodyMap) (wxRsp *QueryRefundResponse, bs []byte, url string, statusCode int, header http.Header, err error) {
	err = bm.CheckEmptyError("nonce_str")
	if err != nil {
		return nil, nil, "", 0, nil, err
	}
	if bm.GetString("refund_id") == util.NULL && bm.GetString("out_refund_no") == util.NULL && bm.GetString("transaction_id") == util.NULL && bm.GetString("out_trade_no") == util.NULL {
		return nil, nil, "", 0, nil, errors.New("refund_id, out_refund_no, out_trade_no, transaction_id are not allowed to be null at the same time")
	}
	if w.IsProd {
		bs, url, statusCode, header, err = w.doProdPost(ctx, bm, refundQuery, nil)
	} else {
		bs, url, statusCode, header, err = w.doSanBoxPost(ctx, bm, sandboxRefundQuery)
	}
	if err != nil {
		return nil, nil, url, statusCode, header, err
	}
	wxRsp = new(QueryRefundResponse)
	if err = xml.Unmarshal(bs, wxRsp); err != nil {
		return nil, nil, url, statusCode, header, fmt.Errorf("xml.UnmarshalStruct(%s)：%w", string(bs), err)
	}
	return wxRsp, bs, url, statusCode, header, nil
}

// 撤销订单
//
//	注意：请在初始化client时，调用 client 添加证书的相关方法添加证书
//	文档地址：https://pay.weixin.qq.com/wiki/doc/api/wxpay_v2/open/chapter4_3.shtml
func (w *Client) Reverse(ctx context.Context, bm gopay.BodyMap) (wxRsp *ReverseResponse, header http.Header, err error) {
	err = bm.CheckEmptyError("nonce_str", "out_trade_no")
	if err != nil {
		return nil, nil, err
	}
	var (
		bs        []byte
		tlsConfig *tls.Config
	)
	if w.IsProd {
		if tlsConfig, err = w.addCertConfig(nil, nil, nil); err != nil {
			return nil, nil, err
		}
		bs, _, _, header, err = w.doProdPost(ctx, bm, reverse, tlsConfig)
	} else {
		bs, _, _, header, err = w.doSanBoxPost(ctx, bm, sandboxReverse)
	}
	if err != nil {
		return nil, header, err
	}
	wxRsp = new(ReverseResponse)
	if err = xml.Unmarshal(bs, wxRsp); err != nil {
		return nil, header, fmt.Errorf("xml.Unmarshal(%s)：%w", string(bs), err)
	}
	return wxRsp, header, nil
}

// GetShortUrl native模式一长码换短码
// 文档地址：https://pay.weixin.qq.com/wiki/doc/api/native.php?chapter=9_9&index=10
func (w *Client) GetShortUrl(ctx context.Context, bm gopay.BodyMap) (wxRsp *ShortUrlResponse, header http.Header, err error) {
	err = bm.CheckEmptyError("appid", "mch_id", "long_url", "nonce_str")
	if err != nil {
		return nil, nil, err
	}

	var bs []byte
	bs, _, _, header, err = w.doProdPost(ctx, bm, shortUrl, nil)
	if err != nil {
		return nil, header, err
	}

	wxRsp = new(ShortUrlResponse)
	if err = xml.Unmarshal(bs, wxRsp); err != nil {
		return nil, header, fmt.Errorf("xml.Unmarshal(%s)：%w", string(bs), err)
	}
	return wxRsp, header, nil
}
