package config

func Init{{ title .Domain }}Domain() {
    di.Wire[{{ title .Domain }}Repository](New{{ title .Domain }}Repository)
    di.Define(New{{ title .Domain }}Management,
        di.Alias[](),
    )
}
