package wallet

type AssetType string

const (
	Coin  AssetType = "coin"
	Token AssetType = "token"
)

func (t AssetType) Valid() bool {
	return t == Coin || t == Token
}
