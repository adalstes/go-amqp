package amqp_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	amqp "github.com/Azure/go-amqp"
)

func Example() {
	// create connection
	conn, err := amqp.Dial("amqps://my-namespace.servicebus.windows.net", &amqp.ConnOptions{
		SASLType: amqp.SASLTypePlain("access-key-name", "access-key"),
	})
	if err != nil {
		log.Fatal("Dialing AMQP server:", err)
	}
	defer conn.Close()

	ctx := context.TODO()

	// open a session
	session, err := conn.NewSession(ctx, nil)
	if err != nil {
		log.Fatal("Creating AMQP session:", err)
	}

	// send a message
	{
		// create a sender
		sender, err := session.NewSender(ctx, "/queue-name", nil)
		if err != nil {
			log.Fatal("Creating sender link:", err)
		}

		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)

		// send message
		err = sender.Send(ctx, amqp.NewMessage([]byte("Hello!")))
		if err != nil {
			log.Fatal("Sending message:", err)
		}

		sender.Close(ctx)
		cancel()
	}

	// continuously read messages
	{
		// create a receiver
		receiver, err := session.NewReceiver(ctx, "/queue-name", &amqp.ReceiverOptions{
			Credit: 10,
		})
		if err != nil {
			log.Fatal("Creating receiver link:", err)
		}
		defer func() {
			ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
			receiver.Close(ctx)
			cancel()
		}()

		for {
			// receive next message
			msg, err := receiver.Receive(ctx)
			if err != nil {
				log.Fatal("Reading message from AMQP:", err)
			}

			// accept message
			if err = receiver.AcceptMessage(context.TODO(), msg); err != nil {
				log.Fatalf("Failure accepting message: %v", err)
			}

			fmt.Printf("Message received: %s\n", msg.GetData())
		}
	}
}

func ExampleConnError() {
	// *ConnErrors are returned when the underlying connection has been closed.
	// this error is propagated to all child Session, Sender, and Receiver instances.

	// create connection
	conn, err := amqp.Dial("amqps://my-namespace.servicebus.windows.net", &amqp.ConnOptions{
		SASLType: amqp.SASLTypePlain("access-key-name", "access-key"),
	})
	if err != nil {
		log.Fatal("Dialing AMQP server:", err)
	}

	ctx := context.TODO()

	// open a session
	session, err := conn.NewSession(ctx, nil)
	if err != nil {
		log.Fatal("Creating AMQP session:", err)
	}

	// create a sender
	sender, err := session.NewSender(ctx, "/queue-name", nil)
	if err != nil {
		log.Fatal("Creating sender link:", err)
	}

	// close the connection before sending the message
	conn.Close()

	// attempt to send message on a closed connection
	err = sender.Send(ctx, amqp.NewMessage([]byte("Hello!")))

	var connErr *amqp.ConnError
	if !errors.As(err, &connErr) {
		log.Fatalf("unexpected error type %T", err)
	}

	// similarly, methods on session will fail in the same way
	_, err = session.NewReceiver(ctx, "/queue-name", nil)
	if !errors.As(err, &connErr) {
		log.Fatalf("unexpected error type %T", err)
	}

	// methods on the connection will also fail
	_, err = conn.NewSession(ctx, nil)
	if !errors.As(err, &connErr) {
		log.Fatalf("unexpected error type %T", err)
	}
}

func ExampleSessionError() {
	// *SessionErrors are returned when a session has been closed.
	// this error is propagated to all child Sender and Receiver instances.

	// create connection
	conn, err := amqp.Dial("amqps://my-namespace.servicebus.windows.net", &amqp.ConnOptions{
		SASLType: amqp.SASLTypePlain("access-key-name", "access-key"),
	})
	if err != nil {
		log.Fatal("Dialing AMQP server:", err)
	}
	defer conn.Close()

	ctx := context.TODO()

	// open a session
	session, err := conn.NewSession(ctx, nil)
	if err != nil {
		log.Fatal("Creating AMQP session:", err)
	}

	// create a sender
	sender, err := session.NewSender(ctx, "/queue-name", nil)
	if err != nil {
		log.Fatal("Creating sender link:", err)
	}

	// close the session before sending the message
	session.Close(ctx)

	// attempt to send message on a closed session
	err = sender.Send(ctx, amqp.NewMessage([]byte("Hello!")))

	var sessionErr *amqp.SessionError
	if !errors.As(err, &sessionErr) {
		log.Fatalf("unexpected error type %T", err)
	}

	// similarly, methods on session will fail in the same way
	_, err = session.NewReceiver(ctx, "/queue-name", nil)
	if !errors.As(err, &sessionErr) {
		log.Fatalf("unexpected error type %T", err)
	}
}

func ExampleDetachError() {
	// *DetachErrors are returned by methods on Senders/Receivers after Close() has been called.
	// it can also be returned if the peer has detached from the link. in this case, the *RemoteErr
	// field should contain additional information about why the peer detached.

	// create connection
	conn, err := amqp.Dial("amqps://my-namespace.servicebus.windows.net", &amqp.ConnOptions{
		SASLType: amqp.SASLTypePlain("access-key-name", "access-key"),
	})
	if err != nil {
		log.Fatal("Dialing AMQP server:", err)
	}
	defer conn.Close()

	ctx := context.TODO()

	// open a session
	session, err := conn.NewSession(ctx, nil)
	if err != nil {
		log.Fatal("Creating AMQP session:", err)
	}

	// create a sender
	sender, err := session.NewSender(ctx, "/queue-name", nil)
	if err != nil {
		log.Fatal("Creating sender link:", err)
	}

	// send message
	err = sender.Send(ctx, amqp.NewMessage([]byte("Hello!")))
	if err != nil {
		log.Fatal("Creating AMQP session:", err)
	}

	// now close the sender
	sender.Close(ctx)

	// attempt to send a message after close
	err = sender.Send(ctx, amqp.NewMessage([]byte("Hello!")))

	var detachErr *amqp.DetachError
	if !errors.As(err, &detachErr) {
		log.Fatalf("unexpected error type %T", err)
	}
}
