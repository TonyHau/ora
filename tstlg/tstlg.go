// Copyright 2015 Rana Ian. All rights reserved.
// Use of this source code is governed by The MIT License
// found in the accompanying LICENSE file.

package tstlg

import "testing"

func New(t *testing.T) Tst {
	return Tst{t}
}

type Tst struct {
	*testing.T
}

func (t Tst) Infof(format string, v ...interface{}) {
	t.Logf("ORA I "+format, v...)
}
func (t Tst) Infoln(v ...interface{}) {
	t.Logf("ORA I ", v...)
}
func (t Tst) Errorf(format string, v ...interface{}) {
	t.Logf("ORA E "+format, v...)
}
func (t Tst) Errorln(v ...interface{}) {
	t.Logf("ORA E ", v...)
}
