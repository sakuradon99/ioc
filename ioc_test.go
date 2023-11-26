package ioc

import (
	ioc "github.com/sakuradon99/ioc/internal"
	"github.com/stretchr/testify/assert"
	"testing"
)

type A struct {
	name    string `value:"name"`
	age     int    `value:"age"`
	address string `value:"address;optional"`
}

func (a A) Name() string {
	return a.name
}

type B struct {
}

type I1 interface {
	Echo1() string
}

type C struct {
}

func (c *C) Echo1() string {
	return "c"
}

type D struct {
}

func (d *D) Echo1() string {
	return "d"
}

type I2 interface {
	Echo2() string
}

type E struct {
}

func (e *E) Echo2() string {
	return "e"
}

type F struct {
}

func (f *F) Echo2() string {
	return "f"
}

type App struct {
	a  *A `inject:""`
	b  *B `inject:";optional"`
	ic I1 `inject:"c"`
	id I1 `inject:"d"`
	i2 I2 `inject:""`
}

func (b *App) TestA() string {
	return b.a.Name()
}

func Test_IOC(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		iocContainer = ioc.NewContainerImpl()
		SetSourceFile("testdata/config.yaml")
		Register[A]()
		Register[C](Name("c"))
		Register[D](Name("d"))
		Register[E](Conditional("#use_e == true"))
		Register[F](Conditional("#use_e != true"))
		Register[App]()

		obj, err := GetObject[App]("")
		if err != nil {
			panic(err)
		}
		assert.NoError(t, err)

		app, ok := obj.(*App)
		assert.Equal(t, true, ok)

		assert.Equal(t, "alice", app.TestA())
		assert.Equal(t, true, app.b == nil)
		assert.Equal(t, "c", app.ic.Echo1())
		assert.Equal(t, "d", app.id.Echo1())
		assert.Equal(t, "e", app.i2.Echo2())
	})

	t.Run("wrong source file", func(t *testing.T) {
		iocContainer = ioc.NewContainerImpl()
		SetSourceFile("testdata/config1.yaml")
		Register[A]()

		_, err := GetObject[A]("")
		assert.Error(t, err)
	})
}
