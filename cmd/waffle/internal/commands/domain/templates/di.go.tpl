package config

func Init{{ title .Domain }}Domain() {
    err := di.Wire[{{ title .Domain }}Repository](New{{ title .Domain }}Repository)
    err = di.Define(New{{ title .Domain }}Management,
        di.Alias[](),
    )
}
