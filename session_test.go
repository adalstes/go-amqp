package amqp

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/adalstes/go-amqp/internal/encoding"
	"github.com/adalstes/go-amqp/internal/frames"
	"github.com/adalstes/go-amqp/internal/mocks"
	"github.com/stretchr/testify/require"
)

func TestSessionClose(t *testing.T) {
	channelNum := uint16(0)
	responder := func(req frames.FrameBody) ([]byte, error) {
		switch req.(type) {
		case *mocks.AMQPProto:
			return []byte{'A', 'M', 'Q', 'P', 0, 1, 0, 0}, nil
		case *frames.PerformOpen:
			return mocks.PerformOpen("container")
		case *frames.PerformBegin:
			b, err := mocks.PerformBegin(uint16(channelNum))
			if err != nil {
				return nil, err
			}
			channelNum++
			return b, nil
		case *frames.PerformEnd:
			// channelNum was incremented
			b, err := mocks.PerformEnd(channelNum-1, nil)
			if err != nil {
				return nil, err
			}
			// channel 0 can never be deleted, however other channels can.
			if channelNum > 1 {
				channelNum = 0
			}
			return b, nil
		default:
			return nil, fmt.Errorf("unhandled frame %T", req)
		}
	}
	netConn := mocks.NewNetConn(responder)

	client, err := New(netConn)
	require.NoError(t, err)
	for i := 0; i < 4; i++ {
		session, err := client.NewSession()
		require.NoErrorf(t, err, "iteration %d", i)
		require.Equalf(t, channelNum-1, session.channel, "iteration %d", i)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		err = session.Close(ctx)
		cancel()
		require.NoErrorf(t, err, "iteration %d", i)
	}
	require.NoError(t, client.Close())
}

func TestSessionServerClose(t *testing.T) {
	responder := func(req frames.FrameBody) ([]byte, error) {
		switch req.(type) {
		case *mocks.AMQPProto:
			return []byte{'A', 'M', 'Q', 'P', 0, 1, 0, 0}, nil
		case *frames.PerformOpen:
			return mocks.PerformOpen("container")
		case *frames.PerformBegin:
			return mocks.PerformBegin(0)
		case *frames.PerformEnd:
			return nil, nil // swallow
		default:
			return nil, fmt.Errorf("unhandled frame %T", req)
		}
	}
	netConn := mocks.NewNetConn(responder)

	client, err := New(netConn)
	require.NoError(t, err)

	session, err := client.NewSession()
	require.NoError(t, err)
	// initiate server-side closing of session
	fr, err := mocks.PerformEnd(0, &encoding.Error{Condition: "closing", Description: "server side close"})
	require.NoError(t, err)
	netConn.SendFrame(fr)
	// wait a bit for connReader to read from the mock
	time.Sleep(100 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	err = session.Close(ctx)
	cancel()
	require.Error(t, err)
	require.Contains(t, err.Error(), "session ended by server")
	require.NoError(t, client.Close())
}

func TestSessionCloseTimeout(t *testing.T) {
	responder := func(req frames.FrameBody) ([]byte, error) {
		switch req.(type) {
		case *mocks.AMQPProto:
			return []byte{'A', 'M', 'Q', 'P', 0, 1, 0, 0}, nil
		case *frames.PerformOpen:
			return mocks.PerformOpen("container")
		case *frames.PerformBegin:
			return mocks.PerformBegin(0)
		case *frames.PerformEnd:
			// sleep to trigger session close timeout
			time.Sleep(1 * time.Second)
			return mocks.PerformEnd(0, nil)
		default:
			return nil, fmt.Errorf("unhandled frame %T", req)
		}
	}
	netConn := mocks.NewNetConn(responder)

	client, err := New(netConn)
	require.NoError(t, err)

	session, err := client.NewSession()
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	err = session.Close(ctx)
	cancel()
	require.Equal(t, context.DeadlineExceeded, err)
	require.NoError(t, client.Close())
}

func TestConnCloseSessionClose(t *testing.T) {
	netConn := mocks.NewNetConn(senderFrameHandlerNoUnhandled(ModeUnsettled))

	client, err := New(netConn)
	require.NoError(t, err)

	session, err := client.NewSession()
	require.NoError(t, err)

	require.NoError(t, client.Close())
	// closing the connection should close all sessions
	select {
	case <-session.done:
		// session was closed
	case <-time.After(500 * time.Millisecond):
		t.Fatal("session wasn't closed")
	}
}

func TestSessionNewReceiverBadOptionFails(t *testing.T) {
	netConn := mocks.NewNetConn(senderFrameHandlerNoUnhandled(ModeUnsettled))

	client, err := New(netConn)
	require.NoError(t, err)

	session, err := client.NewSession()
	require.NoError(t, err)
	recv, err := session.NewReceiver(LinkProperty("", "bad_key"))
	require.Error(t, err)
	require.Nil(t, recv)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	err = session.Close(ctx)
	cancel()
	require.NoError(t, err)
	require.NoError(t, client.Close())
}

func TestSessionNewReceiverBatchingOneCredit(t *testing.T) {
	responder := func(req frames.FrameBody) ([]byte, error) {
		switch tt := req.(type) {
		case *mocks.AMQPProto:
			return []byte{'A', 'M', 'Q', 'P', 0, 1, 0, 0}, nil
		case *frames.PerformOpen:
			return mocks.PerformOpen("container")
		case *frames.PerformBegin:
			return mocks.PerformBegin(0)
		case *frames.PerformEnd:
			return mocks.PerformEnd(0, nil)
		case *frames.PerformAttach:
			return mocks.ReceiverAttach(0, tt.Name, 0, encoding.ModeFirst, nil)
		case *frames.PerformFlow:
			return nil, nil
		default:
			return nil, fmt.Errorf("unhandled frame %T", req)
		}
	}
	netConn := mocks.NewNetConn(responder)

	client, err := New(netConn)
	require.NoError(t, err)

	session, err := client.NewSession()
	require.NoError(t, err)
	recv, err := session.NewReceiver(LinkBatching(true))
	require.NoError(t, err)
	require.NotNil(t, recv)
	require.Equal(t, false, recv.batching, "expected batching disabled with one link credit")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	err = session.Close(ctx)
	cancel()
	require.NoError(t, err)
	require.NoError(t, client.Close())
}

func TestSessionNewReceiverBatchingEnabled(t *testing.T) {
	responder := func(req frames.FrameBody) ([]byte, error) {
		switch tt := req.(type) {
		case *mocks.AMQPProto:
			return []byte{'A', 'M', 'Q', 'P', 0, 1, 0, 0}, nil
		case *frames.PerformOpen:
			return mocks.PerformOpen("container")
		case *frames.PerformBegin:
			return mocks.PerformBegin(0)
		case *frames.PerformEnd:
			return mocks.PerformEnd(0, nil)
		case *frames.PerformAttach:
			return mocks.ReceiverAttach(0, tt.Name, 0, encoding.ModeFirst, nil)
		case *frames.PerformFlow:
			return nil, nil
		default:
			return nil, fmt.Errorf("unhandled frame %T", req)
		}
	}
	netConn := mocks.NewNetConn(responder)

	client, err := New(netConn)
	require.NoError(t, err)

	session, err := client.NewSession()
	require.NoError(t, err)
	recv, err := session.NewReceiver(LinkBatching(true), LinkCredit(10))
	require.NoError(t, err)
	require.NotNil(t, recv)
	require.Equal(t, true, recv.batching, "expected batching enabled with multiple link credits")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	err = session.Close(ctx)
	cancel()
	require.NoError(t, err)
	require.NoError(t, client.Close())
}

func TestSessionNewReceiverMismatchedLinkName(t *testing.T) {
	responder := func(req frames.FrameBody) ([]byte, error) {
		switch req.(type) {
		case *mocks.AMQPProto:
			return []byte{'A', 'M', 'Q', 'P', 0, 1, 0, 0}, nil
		case *frames.PerformOpen:
			return mocks.PerformOpen("container")
		case *frames.PerformBegin:
			return mocks.PerformBegin(0)
		case *frames.PerformEnd:
			return mocks.PerformEnd(0, nil)
		case *frames.PerformAttach:
			return mocks.ReceiverAttach(0, "wrong_name", 0, encoding.ModeFirst, nil)
		default:
			return nil, fmt.Errorf("unhandled frame %T", req)
		}
	}
	netConn := mocks.NewNetConn(responder)

	client, err := New(netConn)
	require.NoError(t, err)

	session, err := client.NewSession()
	require.NoError(t, err)
	recv, err := session.NewReceiver(LinkBatching(true), LinkCredit(10))
	require.Error(t, err)
	require.Nil(t, recv)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	err = session.Close(ctx)
	cancel()
	require.Error(t, err)
	require.NoError(t, client.Close())
}

func TestSessionNewSenderBadOptionFails(t *testing.T) {
	netConn := mocks.NewNetConn(senderFrameHandlerNoUnhandled(ModeUnsettled))

	client, err := New(netConn)
	require.NoError(t, err)

	session, err := client.NewSession()
	require.NoError(t, err)
	snd, err := session.NewSender(LinkProperty("", "bad_key"))
	require.Error(t, err)
	require.Nil(t, snd)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	err = session.Close(ctx)
	cancel()
	require.NoError(t, err)
	require.NoError(t, client.Close())
}

func TestSessionNewSenderMismatchedLinkName(t *testing.T) {
	responder := func(req frames.FrameBody) ([]byte, error) {
		switch req.(type) {
		case *mocks.AMQPProto:
			return []byte{'A', 'M', 'Q', 'P', 0, 1, 0, 0}, nil
		case *frames.PerformOpen:
			return mocks.PerformOpen("container")
		case *frames.PerformBegin:
			return mocks.PerformBegin(0)
		case *frames.PerformEnd:
			return mocks.PerformEnd(0, nil)
		case *frames.PerformAttach:
			return mocks.SenderAttach(0, "wrong_name", 0, encoding.ModeUnsettled)
		default:
			return nil, fmt.Errorf("unhandled frame %T", req)
		}
	}
	netConn := mocks.NewNetConn(responder)

	client, err := New(netConn)
	require.NoError(t, err)

	session, err := client.NewSession()
	require.NoError(t, err)
	snd, err := session.NewSender()
	require.Error(t, err)
	require.Nil(t, snd)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	err = session.Close(ctx)
	cancel()
	require.Error(t, err)
	require.NoError(t, client.Close())
}

func TestSessionNewSenderDuplicateLinks(t *testing.T) {
	netConn := mocks.NewNetConn(senderFrameHandlerNoUnhandled(ModeUnsettled))

	client, err := New(netConn)
	require.NoError(t, err)

	session, err := client.NewSession()
	require.NoError(t, err)
	snd, err := session.NewSender(LinkName("test"))
	require.NoError(t, err)
	require.NotNil(t, snd)
	snd, err = session.NewSender(LinkName("test"))
	require.Error(t, err)
	require.Nil(t, snd)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	err = session.Close(ctx)
	cancel()
	require.NoError(t, err)
	require.NoError(t, client.Close())
}

func TestSessionNewSenderMaxHandles(t *testing.T) {
	netConn := mocks.NewNetConn(senderFrameHandlerNoUnhandled(ModeUnsettled))

	client, err := New(netConn)
	require.NoError(t, err)

	session, err := client.NewSession(SessionMaxLinks(1))
	require.NoError(t, err)
	snd, err := session.NewSender(LinkName("test1"))
	require.NoError(t, err)
	require.NotNil(t, snd)
	snd, err = session.NewSender(LinkName("test2"))
	require.Error(t, err)
	require.Nil(t, snd)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	err = session.Close(ctx)
	cancel()
	require.NoError(t, err)
	require.NoError(t, client.Close())
}

func TestSessionUnexpectedFrame(t *testing.T) {
	netConn := mocks.NewNetConn(senderFrameHandlerNoUnhandled(ModeUnsettled))

	client, err := New(netConn)
	require.NoError(t, err)

	session, err := client.NewSession()
	require.NoError(t, err)

	// this frame is swallowed
	b, err := mocks.EncodeFrame(mocks.FrameSASL, 0, &frames.SASLMechanisms{})
	require.NoError(t, err)
	netConn.SendFrame(b)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	err = session.Close(ctx)
	cancel()
	require.NoError(t, err)
	require.NoError(t, client.Close())
}

func TestSessionInvalidFlowFrame(t *testing.T) {
	netConn := mocks.NewNetConn(senderFrameHandlerNoUnhandled(ModeUnsettled))

	client, err := New(netConn)
	require.NoError(t, err)

	session, err := client.NewSession()
	require.NoError(t, err)

	// NextIncomingID cannot be nil once the session has been established
	b, err := mocks.EncodeFrame(mocks.FrameAMQP, 0, &frames.PerformFlow{})
	require.NoError(t, err)
	netConn.SendFrame(b)

	// wait a bit for connReader to read from the mock
	time.Sleep(100 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	err = session.Close(ctx)
	cancel()
	require.Error(t, err)
	require.NoError(t, client.Close())
}

func TestSessionFlowFrameWithEcho(t *testing.T) {
	nextIncomingID := uint32(1)
	const nextOutgoingID = 2
	responder := func(req frames.FrameBody) ([]byte, error) {
		switch tt := req.(type) {
		case *mocks.AMQPProto:
			return []byte{'A', 'M', 'Q', 'P', 0, 1, 0, 0}, nil
		case *frames.PerformOpen:
			return mocks.PerformOpen("container")
		case *frames.PerformBegin:
			return mocks.PerformBegin(0)
		case *frames.PerformFlow:
			// here we receive the echo.  verify state
			if id := *tt.NextIncomingID; id != nextOutgoingID {
				return nil, fmt.Errorf("unexpected NextIncomingID %d", id)
			}
			if id := tt.NextOutgoingID; id != 0 {
				return nil, fmt.Errorf("unexpected NextOutgoingID %d", id)
			}
			if w := tt.IncomingWindow; w != DefaultWindow {
				return nil, fmt.Errorf("unexpected IncomingWindow %d", w)
			}
			if w := tt.OutgoingWindow; w != DefaultWindow {
				return nil, fmt.Errorf("unexpected OutgoingWindow %d", w)
			}
			return nil, nil
		case *frames.PerformEnd:
			return mocks.PerformEnd(0, nil)
		default:
			return nil, fmt.Errorf("unhandled frame %T", req)
		}
	}
	netConn := mocks.NewNetConn(responder)

	client, err := New(netConn)
	require.NoError(t, err)

	session, err := client.NewSession()
	require.NoError(t, err)

	b, err := mocks.EncodeFrame(mocks.FrameAMQP, 0, &frames.PerformFlow{
		NextIncomingID: &nextIncomingID,
		IncomingWindow: 100,
		OutgoingWindow: 100,
		NextOutgoingID: nextOutgoingID,
		Echo:           true,
	})
	require.NoError(t, err)
	netConn.SendFrame(b)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	err = session.Close(ctx)
	cancel()
	require.NoError(t, err)
	require.NoError(t, client.Close())
}

func TestSessionInvalidAttachDeadlock(t *testing.T) {
	var enqueueFrames func(string)
	responder := func(req frames.FrameBody) ([]byte, error) {
		switch tt := req.(type) {
		case *mocks.AMQPProto:
			return []byte{'A', 'M', 'Q', 'P', 0, 1, 0, 0}, nil
		case *frames.PerformOpen:
			return mocks.PerformOpen("container")
		case *frames.PerformBegin:
			return mocks.PerformBegin(0)
		case *frames.PerformEnd:
			return mocks.PerformEnd(0, nil)
		case *frames.PerformAttach:
			enqueueFrames(tt.Name)
			return nil, nil
		default:
			return nil, fmt.Errorf("unhandled frame %T", req)
		}
	}
	netConn := mocks.NewNetConn(responder)
	client, err := New(netConn)
	require.NoError(t, err)

	session, err := client.NewSession()
	require.NoError(t, err)

	enqueueFrames = func(n string) {
		// send an invalid attach response
		b, err := mocks.EncodeFrame(mocks.FrameAMQP, 0, &frames.PerformAttach{
			Name: "mismatched",
			Role: encoding.RoleReceiver,
		})
		require.NoError(t, err)
		netConn.SendFrame(b)
		// now follow up with a detach frame
		b, err = mocks.EncodeFrame(mocks.FrameAMQP, 0, &frames.PerformDetach{
			Error: &encoding.Error{
				Condition:   "boom",
				Description: "failed",
			},
		})
		require.NoError(t, err)
		netConn.SendFrame(b)
	}
	snd, err := session.NewSender()
	require.Error(t, err)
	require.Nil(t, snd)
	require.NoError(t, client.Close())
}
