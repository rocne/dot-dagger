#!/bin/bash
# @when(os=linux)
# @after(shellrc.base)
export XDG_CONFIG_HOME="${XDG_CONFIG_HOME:-$HOME/.config}"
