package logging

import (
	cou "github.com/nj-eka/fdups/contextutils"
	"github.com/sirupsen/logrus"
)

type ContextFormatter struct {
	BaseFormatter logrus.Formatter
}

func (f *ContextFormatter) Format(e *logrus.Entry) ([]byte, error) {
	if ctx := e.Context; nil != ctx {
		if ops := cou.GetContextOperations(ctx).String(); ops != "" {
			e.Data["ops"] = ops
		}
	}
	return f.BaseFormatter.Format(e)
}

//type ContextFormatter struct {
//	DefaultFields logrus.Fields
//	BaseFormatter logrus.Formatter
//}
//
//func (f *ContextFormatter) Format(e *logrus.Entry) ([]byte, error) {
//	if f.DefaultFields != nil {
//		for key, value := range f.DefaultFields {
//			e.Data[key] = value
//		}
//	}
//	if ctx := e.Context; nil != ctx {
//		if ops := contextutils.GetContextOperations(ctx).String(); ops != "" {
//			e.Data["ops"] = ops
//		}
//	}
//	return f.BaseFormatter.Format(e)
//}
