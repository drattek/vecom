package com.vegusa.ecommerce.exception;

public class SynapseConnectionException  extends RuntimeException {
    private final int offset;

    public SynapseConnectionException(int offset, Throwable cause){
        super("Failed to retrieve page from data source. Offset=" + offset, cause);

        this.offset = offset;
    }

    public int getOffset(){
        return offset;
    }
}
