package diogen

import "gitlab.com/gorib/waffle/app"

func InitDi() (app.App, error) {
    di, err := InitApp()
    if err != nil {
        return nil, err
    }
    application := di.App()
    if appInstance, ok := application.(app.HasCloser); ok {
        appInstance.SetCloser(di.Shutdown())
    }
    return application, nil
}
