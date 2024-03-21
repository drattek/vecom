package com.vegusa.veg_mv_integration_midd.msb.entity;

import jakarta.persistence.*;

@Entity
@Table(name = "VENDTABLE")
public class VendTable
{
 //   @Id
  //  @GeneratedValue(strategy = GenerationType.IDENTITY)
  //  private int id;
    @Id
    @Column(nullable = false)
    private String PAYMTERMID;

    public String getNombre(){ return  this.PAYMTERMID; }

    public  void setNombre(String PAYMTERMID){ this.PAYMTERMID = PAYMTERMID; }
}
