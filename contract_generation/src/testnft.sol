// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.20;
import "./ERC721.sol";

contract MyNFT is ERC721 {
    uint256 private _nextTokenId;
    constructor() ERC721("MyNFT", "MNFT") {
        // start token IDs at 1 if you like
        _nextTokenId = 1;
    }

    @opt @dispatch(0) function safeTransferFrom(address from, address to, uint256 tokenId) public override {
        super.safeTransferFrom(from, to, tokenId);
    }

    @opt @dispatch(1) function safeTransferFrom(address from, address to, uint256 tokenId, bytes memory data) public override {
        super.safeTransferFrom(from, to, tokenId, data);
    }
    
    @opt @dispatch(2) function transferFrom(address from, address to, uint256 tokenId) public override {
        super.transferFrom(from, to, tokenId);
    }

    @opt @dispatch(3)function approve(address to, uint256 tokenId) public override {
        super.approve(to, tokenId);
    }
    
    @opt @dispatch(4) function setApprovalForAll(address operator, bool approved) public override {
        super.setApprovalForAll(operator, approved);
    }

    @opt function balanceOf(address owner) public view override returns (uint256) {
        return super.balanceOf(owner);
    }

    @opt function ownerOf(uint256 tokenId) public view override returns (address) {
        return super.ownerOf(tokenId);
    }

    @opt function getApproved(uint256 tokenId) public view override returns (address) {
        return super.getApproved(tokenId);
    }
    
    @opt function isApprovedForAll(address owner, address operator) public view override returns (bool) {
        return super.isApprovedForAll(owner, operator);
    }

    @opt function name() public view override returns (string memory) {
        return super.name();
    }
    @opt function symbol() public view override returns (string memory) {
        return super.symbol();
    }
    @opt function tokenURI(uint256 tokenId) public view override returns (string memory) {
        return super.tokenURI(tokenId);
    }

    @opt function safeMint(address to) external {
        uint256 tokenId = _nextTokenId;
        _nextTokenId++;

        _safeMint(to, tokenId);
    }

}
