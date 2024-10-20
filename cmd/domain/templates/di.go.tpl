func Init{{ .TitleDomain }}() {
    di.Wire[{{ .TitleDomain }}Repository](New{{ .TitleDomain }}Repository)
    di.Define(New{{ .TitleDomain }}Management,
        di.Alias[](),
    )
}
