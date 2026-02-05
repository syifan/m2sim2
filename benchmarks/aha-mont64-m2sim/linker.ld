/* M2Sim bare-metal linker script for aha-mont64 */
ENTRY(_start)

MEMORY {
    RAM (rwx) : ORIGIN = 0x80000, LENGTH = 1M
}

SECTIONS {
    . = 0x80000;
    
    .text : {
        *(.text.startup)
        *(.text*)
    } > RAM
    
    .rodata : {
        *(.rodata*)
    } > RAM
    
    .data : {
        *(.data*)
    } > RAM
    
    .bss : {
        __bss_start = .;
        *(.bss*)
        *(COMMON)
        __bss_end = .;
    } > RAM
    
    . = ALIGN(16);
    . = . + 0x10000;  /* 64KB stack */
    __stack_top = .;
}
