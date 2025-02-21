package amqp

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/Azure/go-amqp/internal/buffer"
	"github.com/Azure/go-amqp/internal/encoding"
	"github.com/Azure/go-amqp/internal/frames"
	"github.com/Azure/go-amqp/internal/testconn"
	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/require"
)

func fuzzConn(data []byte) int {
	// Receive
	client, err := NewConn(testconn.New(data), &ConnOptions{
		Timeout:     10 * time.Millisecond,
		IdleTimeout: 10 * time.Millisecond,
		SASLType:    SASLTypePlain("listen", "3aCXZYFcuZA89xe6lZkfYJvOPnTGipA3ap7NvPruBhI="),
	})
	if err != nil {
		return 0
	}
	defer client.Close()

	s, err := client.NewSession(context.Background(), nil)
	if err != nil {
		return 0
	}

	r, err := s.NewReceiver(context.Background(), "source", &ReceiverOptions{
		Credit: 2,
	})
	if err != nil {
		return 0
	}

	msg, err := r.Receive(context.Background())
	if err != nil {
		return 0
	}

	if err = r.AcceptMessage(context.Background(), msg); err != nil {
		return 0
	}

	ctx, close := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer close()

	r.Close(ctx)

	s.Close(ctx)

	// Send
	client, err = NewConn(testconn.New(data), &ConnOptions{
		IdleTimeout: 10 * time.Millisecond,
		SASLType:    SASLTypePlain("listen", "3aCXZYFcuZA89xe6lZkfYJvOPnTGipA3ap7NvPruBhI="),
	})
	if err != nil {
		return 0
	}
	defer client.Close()

	s, err = client.NewSession(context.Background(), nil)
	if err != nil {
		return 0
	}

	sender, err := s.NewSender(context.Background(), "source", nil)
	if err != nil {
		return 0
	}

	err = sender.Send(context.Background(), NewMessage(data))
	if err != nil {
		return 0
	}

	ctx, close = context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer close()

	r.Close(ctx)

	s.Close(ctx)

	return 1
}

func fuzzUnmarshal(data []byte) int {
	types := []any{
		new(frames.PerformAttach),
		new(*frames.PerformAttach),
		new(frames.PerformBegin),
		new(*frames.PerformBegin),
		new(frames.PerformClose),
		new(*frames.PerformClose),
		new(frames.PerformDetach),
		new(*frames.PerformDetach),
		new(frames.PerformDisposition),
		new(*frames.PerformDisposition),
		new(frames.PerformEnd),
		new(*frames.PerformEnd),
		new(frames.PerformFlow),
		new(*frames.PerformFlow),
		new(frames.PerformOpen),
		new(*frames.PerformOpen),
		new(frames.PerformTransfer),
		new(*frames.PerformTransfer),
		new(frames.Source),
		new(*frames.Source),
		new(frames.Target),
		new(*frames.Target),
		new(encoding.SASLCode),
		new(*encoding.SASLCode),
		new(frames.SASLMechanisms),
		new(*frames.SASLMechanisms),
		new(frames.SASLChallenge),
		new(*frames.SASLChallenge),
		new(frames.SASLResponse),
		new(*frames.SASLResponse),
		new(frames.SASLOutcome),
		new(*frames.SASLOutcome),
		new(Message),
		new(*Message),
		new(MessageHeader),
		new(*MessageHeader),
		new(MessageProperties),
		new(*MessageProperties),
	}

	for _, t := range types {
		_ = encoding.Unmarshal(buffer.New(data), t)
		_, _ = encoding.ReadAny(buffer.New(data))
	}
	return 0
}

func TestFuzzConnCrashers(t *testing.T) {
	tests := []string{
		0: "\x00\x00\x00\x010000",
		1: "\x00\x00\x00?\x02\x01\x00\x00\x00S@\xc02\x01\xe0/\x04\xb3\x00\x00\x00\aMSSBCBS\x00\x00\x00\x05PLAIN\x00\x00\x00\tANONYMOUS\x00\x00\x00\bEXTERNAL",
		2: "AMQP\x03\x01\x00\x00\x00\x00\x00\x1a0000\x00SD\xc00\x02P0\xa0\x0000000000",
		3: "AMQP\x00\x01\x00\x00\x00\x00\x00\x01\x00\x00\x00S@\xc0108412541625644849\xe0",
		4: "\x00\x00\x00\x1a0000\x00SD\xc000P0\xa0\x0000000000",
		5: "AMQP\x03\x01\x00\x00\x00\x00\x00?\x02\x01\x00\x00\x00S@\xc0" +
			"2\x01\xe0/\x04\xb3\x00\x00\x00\aMSSBCBS\x00\x00\x00" +
			"\x05PLAIN\x00\x00\x00\tANONYMOUS\x00" +
			"\x00\x00\bEXTERNAL\x00\x00\x00\x1a\x02\x01\x00\x00\x00" +
			"SD\xc0\r\x02P\x00\xa0\bWelcome!AMQ" +
			"P\x00\x01\x00\x00\x00\x00\x00G\x02\x00\x00\x00\x00S\x10\xc0:\n\xa1" +
			"$83a29bedd884468ba2e" +
			"37f3017eeab1d_G29@p\x00" +
			"\x00\x02\x00`\x00\x01p\x00\x03\xa9\x80@@@@@\x00\x00\x00\x1f" +
			"\x02\x00\x00\x00\x00S\x11\xc0\x12\b`\x00\x00R\x01p\x00\x00\x13\x88" +
			"R\x01R\xff@@@\x00\x00\x00d\x02\x00\x00\x00\x00S\x12\xc0W" +
			"\x0e\xa1(oJnNPGsiuzytMOJPa" +
			"twtPilfsfykSBGplhxtx" +
			"VSGCB@P\x01\x00S(\xc0\x12\v\xa1\x05/tes" +
			"t@@@@@@@@@@@@@C\x80\x00\x00\x00\x00" +
			"\x00\x04\x10\x00@@@\x00\x00\x01y\x02\x00\x00\x00\x00S\x14\xc0\x1d" +
			"\vCC\xa0\x10F>\xc6\\\x06&\xfaE\x9c\x03\xa8\x8e\xe7\x83\xe3" +
			";C@B@@@@A\x00Sp\xc0\n\x05@@pH\x19" +
			"\b\x00@C\x00Sr\xc1\\\x06\xa3\x13x-opt-en" +
			"queued-time\x83\x00\x00\x01[\x9c_)\xd1" +
			"\xa3\x15x-opt-sequence-num" +
			"ber\x81\x00\x00\x00\x00\x00\x00\x03x\xa3\x12x-opt-" +
			"locked-until\x83\x00\x00\x01[\x9c_\x9f" +
			"\x11\x00Ss\xc0H\r\xa1$5e84053f-81" +
			"c9-49fc-ae42-ff0ab35" +
			"3d998@@\xa1\x14Service Bus" +
			" Explorer@@@@@@@@@\x00S" +
			"t\xc18\x04\xa1\vMachineName\xa1\x0fW" +
			"IN-37U7RVPH3B1\xa1\bUser" +
			"Name\xa1\rAdministrator\x00" +
			"Su\xa0P<?xml version=\"1" +
			".0\" encoding=\"utf-8\"" +
			"?>\r\n<message>Hi mate" +
			", how are you?</mess" +
			"age>",
		6: "AMQP\x03\x01\x00\x00\x00\x00\x00?\x02\x01\x00\x00\x00S@\xc0" +
			"2\x01\xe0/\x04\xb3\xf8\x00\x00\aMSSBCBm\x00\x00\x00" +
			"\x05PLA\xff\x00\x00\x00\x00\tANONYMOUS\x00" +
			"\x00\x00\b\x14\nEXTERNAL",
		7: "AMQP\x03\x01\x00\x00\x00\x00\x00?\x02\x01\x00\x00\x00S@\xc0" +
			"2\x01\xe0/\x04\xb3\x00\x00\x00\aMSSBCBS\x00\x00\x00" +
			"\x05PLAIN\x00\x00\x00\tANONYMOUS\x00" +
			"\x00\x00\bEXTERNAL\x00\x00\x00\x1a\x02\x01\x00\x00\x00" +
			"SD\xc0\r\x02P\x00\xa0\bWelcome!AMQ" +
			"P\x00\x01\x00\x00\x00\x00\x00G\x02\x00\x00\x00\x00S\x10\xc0:\n\xa1" +
			"$83a29bedd884468ba2e" +
			"37f3017eeab1d_G29@p\x00" +
			"\x00\x02\x00`\x00\x01p\x00\x03\xa9\x80@@@@@\x02",
		8: "AMQP\x03\x01\x00\x00\x00\x00\x00?\x02\x01\x00\x00\x00S@\xc0" +
			"2\x01\xe0/\x04\xb3\x00\x00\x00\aMSS \x00\x00\x00\x00\x00\x00" +
			"\x05PLAIN�^�~\x00\x00\x00\tAN" +
			"\xcfNYMOUS\x00\x00\x00\bEXT\xf1\xf1I\xdf\xed",
		9: "AMQP\x03\x01\x00\x00\x00\x00\x00?\x02\x01\x00\x00\x00S@\xc0" +
			"2\x01\xe0/\x04\xb3\x00\x00\x00\aMSSBCBS\x00\x00\x00" +
			"\x05PLAIN\x00\x00\x00\tANONYMOUS\x00" +
			"\x00\x00\bEXTERNAL\x00\x00\x00\x1a\x02\x01\x00\x00\x00" +
			"SD\xc0\r\x02P\x00\xa0\bWelcome!AMQ" +
			"P\x00\x01\x00\x00\x00\x00\x00G\x02\x00\x00\x00\x00S\x10\xc0:\n\xa1" +
			"$83a29bedd884468ba2e" +
			"37f3017eeab1d_G29@p\x00" +
			"\x00\x02\x00`\x00\x01p\x00\x03\xa9\x80@@@@@\x00\x00\x00\x1f" +
			"\x02\x00\x00\x00\x00S\x11\xc0\x12\b`\x00\x00R\x01p\x00\x00\x13\x88" +
			"R\x01R\xff@@@\x00\x00\x02\x00S\x12\xa1JNPuzy" +
			"MPawtPilffySBhxtxVGC" +
			"B@\x00\xc0\x05/ts@@@@C\x80\x00\x00\x04@\x00\x00" +
			"\x01y\x02\x00\x14\xc0CC\x10>\xc6\\\x9c\xa8\xe7\x83;@@@" +
			"@pHC\x00S\xc1\\\xa3\x13xopenqud-t" +
			"me\x83\x00\x01\xa3\x15-o-equneubr\x00\x00" +
			"\x00\x00\x03xxp-lced-util\x83\x00\x01\x9c" +
			"\x9f\r\xa1$5e0589-f-4fb339@" +
			"@\xa1\x14ervuExplor@@@@@\xc1a" +
			"cNIN3VP\xa1UserN\xa1Amitao" +
			"u?m n=\". cin=\"ut-?>m" +
			"ssagHimae arey/mess>",
		10: "AMQP\x03\x01\x00\x00\x00\x00\x00?\x02\x01\x00\x00\x00S@\xc0" +
			"2\x01\xe0/\x04\xb3\xa9\x80@@@@@\x00\x00\x00\x1f\x02\x00\x00" +
			"\x00\x00S\x11\xc0\x12\b`\x00\x00R\x01p\x00\x00\x13\x88R\x01R" +
			"\xff@@@\x00\x00\x00d\x00\x00S\x12\xc0",
		11: "AMQP\x03\x01\x00\x00\x00\x00\x00?\x02\x01\x00\x00\x00S@\xc0" +
			"2\x01\xe0/\x04\xb3\x00\x00\x00\aMSSBCBS\x00\x00\x00" +
			"\x05PLAIN\x00\x00\x00\tANONYMOUS\x00" +
			"\x00\x00\bEXTERNAL\x00\x00\x00\x1a\x02\x01\x00\x00\x00" +
			"SD\xc0\r\x02P\x00\xa0\bWelcome!AMQ" +
			"P\x00\x01\x00\x00\x00\x00\x00G\x02\x00\x00\x00\x00S\x10\xc0:\n\xa1" +
			"$83a29bedd884468ba2e" +
			"37f3017eeab1d_G29@p\x00" +
			"\x00\x02\x00`\x00\x01p\x00\x03\xa9\x80@@@@@\x00\x00\x00\x1f" +
			"\x02\x00\x00\x00\x00S\x11\xc0\x12\b`\x00\x00R\x01p\x00\x00\x13\x88" +
			"R\x01R\xff@@@\x00\x00S(oJNPsiMPf" +
			"sfkSGlxxVGCB@P\x01\x00\xc0\x12\xa1\x05" +
			"/st@@@@\x80\x00\x00\x10\x00@@\x00\x00\x00S\x14\xc0" +
			"\x1d\vC\xa0\x10>\\\xfa\xa8\x8eC@@A\x00\xc0\n@\b@" +
			"\x00\x06\xa3x-oenqe-ti\x00\x01[_ѣx" +
			"-p-senenue\x00\x00\x00\x00\x03x\xa3\x12pt" +
			"-lked-tl\x83\x00\x01\x9c\x11\x00Ss\xc05e8" +
			"41c9-4c-e42-fab3d\xa1\x14e" +
			"rvsEpr@@@@@@\x00t\xc1\x04\xa1Mhi" +
			"n\xa1\x0fN3RB\bUsNminrato\xa0<" +
			"?m vro=\"10ncodig\"utf" +
			"?\r<ea mateho re o?<m" +
			"ssag>",
		12: "AMQP\x03\x01\x00\x00\x00\x00\x00?\x02\x01\x00\x00\x00S@\xc0" +
			"2\x01\xe0/\x04\xb3\x00\x00\x00\aMSSBCBS\x00\x00\x00" +
			"\x05PLAINlf�\t��s\x00" +
			"?\x02\x00\x00\x00\x00ShxtxVSGCB@P\x01\x00" +
			"Q",
		13: "AMQP\x03\x01\x00\x00\x00\x00\x00?\x02\x01\x00\x04\x00S@\xc0" +
			"1\x01\xe0/\x04\xb3\x00\x00\x00\aMSSBCB\xff\x00\x00\x00" +
			"\x05PLAIN\xfa\x00\x00\tAN\xcfNYMOUS\x00" +
			"\x00\x00\bE\xef\xbf\x02\x00\fU ",
		14: "AMQP\x03\x01\x00\x00\x00\x00\x00?\x02\x01\x00\x00\x00S@\xc0" +
			"2\x01\xe0/\x04\xb3\x00\x00\x00\aMSSBCBS\x00\x00\x00" +
			"\x05PLAIN\x00\x00\x00\tANONYMOUR\xff" +
			"\xff\xed\bEXTERNAL\x00\x00\x00\x1a\x02\x01\x00\x00\x00" +
			"SD\xc0\r\x02P\x00\xa0\bWelcome!AMQ" +
			"P\x00\x01\x00\x00\x00\x00\x00G\x02\x00\x00\x00\x00S\x10\xc0:\n\xa1" +
			"$83a29bedd884468ba2e" +
			"37f3017eeab1d_G29@p\x00" +
			"\x00\x02\x00`\x00\x01p\x00\x03\xa9\x80@@@@@\x00\x00\x00\x1f" +
			"\x02\x00\x00\x00\x00S\x11\xc0\x12\b`\x00\x00R\x01p\x00\x00\x13\x88" +
			"R\x01R\xff@@@\x00\x00\x00d\x02\x00\x00\x00\x00S\x12\xc0W" +
			"\x0e\xa1(oJnNPGsiuzytMOJPa" +
			"twtPilfsfykSBGplhxtx" +
			"VSGCB@P\x01\x00S(\xc0\x12\v\xa1\x05/tes" +
			"t@@@@@@@@@@@C\x80\x00\x00\x00\x00\x00\x04" +
			"\x10\x00@@@\x00\x00\x01y\x02\x00\x00\x00\x00S\x14\xc0\x1d\vC" +
			"C\xa0\x10F>\xc6\\\x06&\xfaE\x9c\x03\xa8\x8e\xe7\x83\xe3;C" +
			"@B@@@@A\x00Sp\xc0\n\x05@@pH\x19\b\x00" +
			"@C\x00Sr\xc1\\\x06\xa3\x13x-opt-enqu" +
			"eued-time\x83\x00\x00\x01[\x9c_)ѣ\x15" +
			"x-opt-sequence-numbe" +
			"r\x81\x00\x00\x00\x00\x00\x00\x03x\xa3\x12x-opt-lo" +
			"cked-until\x83\x00\x00\x01[\x9c_\x9f\x11\x00" +
			"Ss\xc0H\r\xa1$5e84053f-81c9" +
			"-49fc-ae42-ff0ab353d" +
			"998@@\xa1\x14Service Bus E" +
			"xplorer@@@@@@@@@\x00St\xc1" +
			"8\x04\xa1\vMachineNam",
		15: "AMQP\x03\x01\x00\x00\x00\x00\x00?\x02\x01\x00\x00\x00S@\xc0" +
			"2\x01\xe0/\x04\xb3\x00\x00\x00\aMSSBCBS\x00\x00\x00" +
			"\x05PLAIN\x00\x00\x00\tANONYMOUS\x00" +
			"\x00\x00\bEXTERNAL\x00\x00\x00\x1a\x02\x01\x00\x00\x00" +
			"SD\xc0\r\x02P\x00\xa0\bWelcome!A2Q" +
			"P\x00\x01\x00\x00\x00\x00\x00G\x02\x00\x00\x00\x00S\x10\xc0:\n\xa1" +
			"$83a29bedd884468ba2e" +
			"37f3017eeab1d_G29@p\x00" +
			"\x00\x02\x00`\x00\x01p\x00\x03\xa9\x80@@@@@\x00\x00\x00\x1f" +
			"\x02\x00\x00\x00\x00S\x11\xc0\x12\b`\x00\x00R\x01p\x00\x00\x13\x88" +
			"R\x01R\xff@@@\x00\x00\x00d\x02\x00\x00\x00\x00S\x12\xc0W" +
			"\x0e\xa1(oJnNPGsiuzytMOJPa" +
			"twtPilfsfykSBGplhxtx" +
			"VSGCB@P\x01\x00S(\xc0\"\xd9\aTERNA" +
			"L\x00\x00t@@@@@@@\x17\r\r\x1a@@@@@" +
			"@C\x80\x00\x00\x01[\x9c\x00\x00\x00\x00\x04\x10\x00@-@\x00\x00" +
			"\x01y\x02\x00\x00\x00\x00S\x14\xc0\x1d\vCC\xa0\x10F>\xc6\\" +
			"\x06&\xfaE\x9c\x03\xa8\x8e\xe7\x83\xe3;C@B@@@A\x00" +
			"Sp\xc0\n\x05@@pH\x19\b\x00@C\x00r\xc1\\\xa3\x13" +
			"xop-enqueed-im\x83\x00\x00\x01[\x9c" +
			"_-ѣ\x15x-opt-squnce-nu" +
			"mber\x81\x00\x00\x00\x00\x00\x00\x03x\xa3\x12x@opt" +
			"-locked-until\x83\x00\x00\x01[\x9c_" +
			"\x9f\x11\x00Ss\xc0\r\xa1$58405f-81c9" +
			"-4fc-ae42-ff0ab353d9" +
			"98@@\xa1\x14Servie ",
		16: "\x00\x00\x0000000\x00S\x17\xc00\x01000000" +
			"00000000000000000000" +
			"00000000000000000000" +
			"000",
		17: "AMQP\x03\x01\x00\x00\x00\x00\x00?\x02\x01\x00\x00\x00S@\xc0" +
			"2\x01\xe0/\x04\xb3\x00\x00\x00\aMSSBCBS\x00\x00\x00" +
			"\x05PLAIN\x00\x00\x00\tANONYMOUS\x00" +
			"\x00\x00\bEXTERNAL\x00\x00\x00\x1a\x02\x01\x00\x00\x00" +
			"SD\xc0\r\x02P\x00\xa0\bWelcome!AMQ" +
			"P\x00\x01\x00\x00\x00\x00\x00G\x02\x00\x00\x00\x00S\x10\xc0:\n\xa1" +
			"$83a29bedd884468ba2e" +
			"37f301\"eeab1d_G29@p\x00" +
			"\x00\x00\x00`\x00\x01p\x00\x03\xa9\x80@@@@@\x00\x00\x00\x1f" +
			"\x02\x00\x00\x00\x00S\x11\xc0\x12\b`\x00\x00R\x01p\x00\x00\x13\x88" +
			"R\x01R\xff@@@\x00\x00\x00d\x02\x00\x00\x00\x00S\x12\xc0W" +
			"\x0e\xa1(oJnNPGsiuzytMOJPa" +
			"twtPilfsfykSBGplhxtx" +
			"VSGCB@P\x01\x00S(\xc0\x12\v\xa1\x05/tes" +
			"t@@@@@@@@@@@@@C\x80\x00\x00\x00\x00" +
			"\x00\x04\x10\x00@@@\x00\x00\x01y\x00\x00\x00\x00S\x14\xc0\x1dC" +
			"C\xa0\x10F>\xc6\\\x06&\xfaE\x9c\x03\xa8\x8e\xe7\x83\xe3C@" +
			"B@@@@\x00Sp\xc0\n\x05@@p\x19\b\x00@C\x00" +
			"Sr\xc1\\\x06\xa3\x13x-opt-enqueue" +
			"d-time\x83\x00\x00\x01[\x9c)ѣ\x15x-op" +
			"t-equence-nmbe\x81\x00\x00\x00\x00\x00" +
			"\x03x\xa3\x12x-opt-\xe6ocke-unti" +
			"l\x83\x00\x00\x01\x9c_\x9f\x11\x00SsH\r\xa1$5e84" +
			"053f-81c9-49fc-ae42-" +
			"ff0b353d998@\xa1\x14Servic" +
			" Bus Explrer@@@@@@@@" +
			"@\x00St\xc18\x04\xa1\vMachineName" +
			"\xa1\x0fWIN-37U7RVPH3B1\xa1Us" +
			"erName\xa1Administrator" +
			"\x00Su\xa0P<?xml verion=\"",
		18: "\x00\x00\x00\x1f0000\x00S\x13\xc00\b`00000" +
			"00000000000",
		19: "AMQP\x03\x01\x00\x00\x00\x00\x00?\x02\x01\x00\x00\x00S@\xc0" +
			"2\x01\xe0/\x04\xb3\x00\x00\x00\aMSSBCBS\x00\x00\x00" +
			"\x05PLAIN\x00\x00\x00\tANONYMOUS\x00" +
			"\x00\x00\bEXTERNAL\x00\x00\x00\x1a\x02\x01\x00\x00\x00" +
			"SD\xc0\r\x02P\x00\xa0\bWelcome!AMQ" +
			"P\x00\x01\x00\x00\x00\x00\x00G\x02\x00\x00\x00\x00S\x10\xc0:\x00\x01" +
			"$8ֽ\xbf\xefѿｿｿ\xef\xef\xbf\xd5\xef\xcd" +
			"\xbd��e\x85a\xe8\x03d_\xe629@p\x00" +
			"\x00\x02\x00`\x00\x01p\x00\x03\xa9\x802dcfbb599" +
			"75f217c445f95634d7c0" +
			"250afe7d8316a70c47db" +
			"a99ff94167ab74349729" +
			"ce1d2bd5d161df27a6a6" +
			"e7cba1e63924fcd03134" +
			"abdad4952c3c409060d7" +
			"ca2ee4e5f4c647c3edee" +
			"7ad5aa1cbbd341a8a372" +
			"ed4f4db1e469ee250a4e" +
			"fcc46de1aa52a7e22685" +
			"d0915b7aae075defbff1" +
			"529d40a04f250a2d4a04" +
			"6c36c8ca18631cb05533" +
			"4625c4919072a8ee5258" +
			"efb4e6205525455f428f" +
			"63aeb62c68de9f758ee4" +
			"b8c50a7d669ae00f8942" +
			"5868f73e894c53ce9b96" +
			"4dff34f42b9dc2bb0351" +
			"9fbc169a397d25197cae" +
			"5bc50742f3808f474f2a" +
			"dd8d1a0281359043e0a3" +
			"95705fbc0a89293fa2a5" +
			"ddfe6ae5416e65c0a5b4" +
			"eb83320585b33b26072b" +
			"c99c9c1948a6a271d645" +
			"17a433728974d0ff4586" +
			"a42109d6268f9961a590" +
			"8d6f2d198875b02ae786" +
			"6fff3a9361b41842a35d" +
			"c9477ec32da542b706f8" +
			"478457649ddfda5dfab1" +
			"d45aa10efe12c3065566" +
			"541ebdc2d1db6814826f" +
			"0cc9e3642e813408df3e" +
			"baa3896bb2777e757dc3" +
			"dbc1d28994a454fcb8d7" +
			"6bc5914f29cfc05dc89f" +
			"8c734315def58d4d6b0b" +
			"0136ccd3c05178155e30" +
			"fcb9f68df9104dc96e06" +
			"58fa899c0058818da5ec" +
			"88a723558ae3a6f2f8f5" +
			"23e5af1a73a82ab16198" +
			"c7ba8341568399d8013f" +
			"c499e6e7ef61cb8654b4" +
			"8b88aa2a931dc2cdcf24" +
			"5686eed9c8355d620d5e" +
			"91c1e878a9c7da655e3f" +
			"29d9b7c3f44ad1c70890" +
			"eb5f27ca28efff76420c" +
			"d4e3cebd5c788536ddd3" +
			"65f7ad1dbb91588d5861" +
			"2e43b0460de9260d5f78" +
			"0a245bc8e1a83166df1f" +
			"3a3506d742c268ab4fc1" +
			"0c6e04bca40295da0ff5" +
			"420a199dd2fb36045215" +
			"138c4a2a539ceccc382c" +
			"8d349a81e13e84870894" +
			"7c4a9e85d861811e75d3" +
			"23896f6da3b2fa807f22" +
			"bcfc57477e487602cf8e" +
			"973bc925b1a19732b00d" +
			"15d38675313a283bbaa7" +
			"5e6793b5af11fe2514bd" +
			"a3abe96cc19b0e58ddbe" +
			"55e381ec58c31670fec1" +
			"184d38bbf2d7cde0fcd2" +
			"9e907e780d30130b98e0" +
			"c9eec44bcb1d0ed18dfd" +
			"a2a64adb523da3102eaf" +
			"e2bd3051353d8148491a" +
			"290308ed4ec3fa5da578" +
			"4b481e861360c3b670e2" +
			"56539f96a4c4c4360d0d" +
			"40260049035f1cfdacb2" +
			"75e7fa847e0df531b466" +
			"141ac9a3a16e78659475" +
			"72e4ab732daec23aac6e" +
			"ed1256d796c4d58bf699" +
			"f20aa4bbae461a16abbe" +
			"9c1e9@@@@@",
		20: "\x00\x00\x0000000\x00S\x18\xc00\x01000000" +
			"00000000000000000000" +
			"00000000000000000000" +
			"000",
		21: "",
		22: "SSSBCB\x00\x00\x00S\x12\xc0W\x0e\xa1(pqa\xbd" +
			"\xbfｿ\xef\x17\x1a\r\r\x15Xhؿ\xbdJ\xf0\xbf\xbd\xef" +
			"ǽDpHYjxeUBrVfdwCB@P" +
			"\x01\x00S(\xc0\x1a\v\xa1\x05/test@@@@@@" +
			"BS\x00\x00\x00\x05PL@@@@@@@C\x80\x00\x00\x00" +
			"\x00\x00\x04\x10\x00@@@",
		23: "\x00\x00\x00d\x02\x00#\x00\x00S\x12\xc0W\x0e\xa1crypt" +
			"o/des: invalid key s" +
			"ize (pqa\xbd\xdb\xf1\xbd\xbf\xef%\xbd\xdbQ\xbd\xbf" +
			"\xef%\xbf\xbdJ\xf0\xbf\xbf\xf1\xbd\xbd\xdb\xf1\xbf\xf1\xba\xbf\xf1\xbd\xbd" +
			"\xdb\xf1\xbd\xbfwCB@P\x01\x00S(\xc0\x1a\v\xa1\x05/t" +
			"est@@@@@@@@@@@@@C\x80\x00\x00" +
			"\x00\x80\x00\x04\x10\x00@@@",
		24: "\x00\x00\x00d\x02\x00\x00\x00\x00S\x12\xc0\x00\x0e\xa1(p\xbd\xbf\xef" +
			"\xbd\xdf\uf03d\xbfｿｿｿ\xef\xff\xff\xff\x80" +
			"\xbd\xbfｿｿ\x00\x02BrXfdw`@CB" +
			"@P\x01\x00`S(\xc0\x12\v\xa1\x05./est`@@" +
			"@`\x80@@@@@\x00P\xff\x00\x00\x00@@@`@\x00",
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			end := leaktest.Check(t)
			require.Zero(t, fuzzConn([]byte(tt)))
			end()
		})
	}
}

func TestFuzzMarshalCrashers(t *testing.T) {
	tests := []string{
		0:  "\xc1\x000\xa0\x00S0",
		1:  "\xf0S\x13\xc0\x12\v@`@@`\v@```@@@",
		2:  "\xe000\xb0",
		3:  "\xc1\x000\xe000R",
		4:  "\xe000S",
		5:  "\x00\xe000R",
		6:  "\xe000\x83",
		7:  "\x00\x00\xe000S",
		8:  "\xe000R",
		9:  "\x00\xe000S",
		10: "\xc1\x000\xe000S",
		11: "\xc1\x000\x00\xe000S",
		12: "\xc1\x000\x00\xe000R",
		13: "\x00\x00\xe000R",
		14: "\xe000\xb1",
		15: "\xc1\x00%\xd0\x00\x00\x00M\xe2\x00\x00\x01\x00S\x1d\xd0\x00\x00\x00A" +
			"\x00\x00\x00\x03\xa3\x10amqp:link:stol" +
			"en\xa1\x0foo\xb1\xdefoo descript" +
			"ion\xc1\x18\x04\xa1\x05other\xa1\x04info\xa1" +
			"\x03andq\x00\x00\x03k",
		16: "\xd1\x00\x00\x00M\x00S\x1d\xd0\x00S\x1d\xd0\x00\x00\x00A\x00\x80\x00" +
			"\x03\xa3\x10amqp:link:stolen\xa1" +
			"\x19foo description\xc1\x18\x04\xa1" +
			"\x05other\xa1\x04info\xa1\x03andU\x00\x00" +
			"\x03k",
		17: "\xf0\x00\x00\x00\x01@\x00TRUE\x00",
		18: "\xf0\x00\x00\x00\x00\x10RTRT",
		19: "\x00p\x00inp\xf0\x00\x00\x00\x01p\x00inp",
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			fuzzUnmarshal([]byte(tt))
		})
	}
}

func testDirFiles(t *testing.T, dir string) []string {
	finfos, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	var fullpaths []string
	for _, finfo := range finfos {
		fullpaths = append(fullpaths, filepath.Join(dir, finfo.Name()))
	}

	return fullpaths
}

func TestFuzzConnCorpus(t *testing.T) {
	if os.Getenv("TEST_CORPUS") == "" {
		t.Skip("set TEST_CORPUS to enable")
	}

	for _, path := range testDirFiles(t, "internal/encoding/testdata/fuzz/conn/corpus") {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			defer leaktest.Check(t)()
			fuzzConn(data)
		})
	}
}

func TestFuzzMarshalCorpus(t *testing.T) {
	if os.Getenv("TEST_CORPUS") == "" {
		t.Skip("set TEST_CORPUS to enable")
	}

	for _, path := range testDirFiles(t, "internal/encoding/testdata/fuzz/marshal/corpus") {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}

			fuzzUnmarshal(data)
		})
	}
}
