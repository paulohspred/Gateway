# Toolchain

O módulo principal declara **Go 1.27** (`go 1.27.0`) e o workflow do Gateway usa **Go 1.27.1**.

Após a limpeza bridge-first, o core não possui dependências Go externas: TCP, concorrência, I/O, configuração e observabilidade usam a biblioteca padrão. Dependências adicionais só devem entrar quando um endpoint provider realmente exigir e devem ser fixadas/revisadas individualmente.
