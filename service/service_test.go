package service

import (
	"context"
	"testing"
)

func TestService_SaveAndPublish(t *testing.T) {
	type fields struct {
		l           *logger.UPPLogger
		draftAPI    *draft.API
		notifierAPI *notifier.API
	}
	type args struct {
		ctx  context.Context
		uuid string
		hash string
		body map[string]interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Service{
				l:           tt.fields.l,
				draftAPI:    tt.fields.draftAPI,
				notifierAPI: tt.fields.notifierAPI,
			}
			if err := s.SaveAndPublish(tt.args.ctx, tt.args.uuid, tt.args.hash, tt.args.body); (err != nil) != tt.wantErr {
				t.Errorf("SaveAndPublish() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
