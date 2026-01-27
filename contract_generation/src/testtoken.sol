// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.20;
import "./ERC20.sol";

contract TestToken is ERC20 {
    constructor(uint initialSupply) ERC20("TestToken", "TEST") {
        _mint(msg.sender, initialSupply);
    }

    @opt @dispatch(1) function transfer(address to, uint256 value) public override returns (bool) {
        return super.transfer(to, value);
    }

    @opt @dispatch(2)function transferFrom(address from, address to, uint256 value) public override returns (bool) {
        return super.transferFrom(from, to, value);
    }

    @opt function name() public view override returns (string memory) {
        return super.name();
    }

    @opt @dispatch(3) function approve(address spender, uint256 value) public override returns (bool) {
        return super.approve(spender, value);
    }

    @opt function totalSupply() public view override returns (uint256) {
        return super.totalSupply();
    }

    @opt function decimals() public view override returns (uint8) {
        return super.decimals();
    }

    @opt @dispatch(0) function balanceOf(address account) public view override returns (uint256) {
        return super.balanceOf(account);
    }

    @opt function symbol() public view override returns (string memory) {
        return super.symbol();
    }

    @opt function allowance(address owner, address spender) public view override returns (uint256) {
        return super.allowance(owner, spender);
    }

}
